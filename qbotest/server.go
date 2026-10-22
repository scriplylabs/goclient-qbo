// Package qbotest is a fake QuickBooks Online API for tests of code that uses
// package quickbooks. It keeps entities in memory, records every request, and
// can be told to fail the next request with a fault QuickBooks returns.
//
//	srv, client := qbotest.NewServer(t)
//	srv.FailNext("journalentry", qbotest.NewAccountPeriodClosedFault())
//	_, err := client.CreateJournalEntry(input, quickbooks.WithRequestID(key))
//	body := srv.ListRequestsTo("journalentry")[0].Body
//
// The fault bodies are copies of what a QuickBooks sandbox returned; the
// sandbox tests in package quickbooks (go test -tags sandbox) check that they
// still match. qbotest doesn't apply QuickBooks' business rules (balanced
// journal entries, book close dates, required fields); queue a fault with
// FailNext to test those paths.
package qbotest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	quickbooks "github.com/scriplylabs/goclient-qbo"
)

// Realm is the company ID of the fake company.
const Realm = "1234567890"

// Request is a request the server received.
type Request struct {
	Method string
	// Path is relative to /v3/company/{realm}/, for example "journalentry" or "account/12".
	Path  string
	Query map[string][]string
	// RequestID is the requestid query parameter, or "".
	RequestID string
	// Body is the request body exactly as sent.
	Body []byte
	// Status is the HTTP status the server answered with, or 0 for a request
	// that hung until the client gave up.
	Status int
}

// Fault is a failure the server can be told to return; see FailNext.
type Fault struct {
	status int
	body   string
	hang   bool
}

// NewValidationFault returns a 400 NewValidationFault with the given code, message and detail.
func NewValidationFault(code, message, detail string) Fault {
	body, _ := json.Marshal(map[string]any{
		"Fault": map[string]any{
			"Error": []map[string]string{{"Message": message, "Detail": detail, "code": code, "element": ""}},
			"type":  "ValidationFault",
		},
		"time": formatNow(),
	})

	return Fault{status: http.StatusBadRequest, body: string(body)}
}

// NewAccountPeriodClosedFault returns the ValidationFault (code 6200) QuickBooks sends
// for a transaction dated on or before the company's book close date.
func NewAccountPeriodClosedFault() Fault {
	return NewValidationFault("6200", "Account Period Closed",
		"The account period has closed. Txn Date=06/15/2026 is before book closing date=06/30/2026")
}

// NewAuthenticationFault returns the 401 QuickBooks sends for an invalid or
// expired access token. Unlike other faults it uses lowercase keys and gives
// the time in epoch milliseconds.
func NewAuthenticationFault() Fault {
	return Fault{status: http.StatusUnauthorized, body: fmt.Sprintf(`{"warnings":null,"intuitObject":null,`+
		`"fault":{"error":[{"message":"message=AuthenticationFailed; errorCode=003200; statusCode=401",`+
		`"detail":"Token expired","code":"3200","element":null}],"type":"AUTHENTICATION"},`+
		`"report":null,"queryResponse":null,"batchItemResponse":[],"attachableResponse":[],`+
		`"syncErrorResponse":null,"requestId":null,"time":%d,"status":null,"cdcresponse":[]}`, time.Now().UnixMilli())}
}

// NewServerErrorFault returns a 500 with a SystemFault body.
func NewServerErrorFault() Fault {
	body, _ := json.Marshal(map[string]any{
		"Fault": map[string]any{
			"Error": []map[string]string{{
				"Message": "An application error has occurred while processing your request",
				"Detail":  "System Failure Error: An unexpected error occurred while accessing or saving your data.",
				"code":    "10000",
			}},
			"type": "SystemFault",
		},
		"time": formatNow(),
	})

	return Fault{status: http.StatusInternalServerError, body: string(body)}
}

// NewHangFault returns a fault that never answers: the request waits until the client
// gives up (set the client's Client.Timeout) or the server is closed.
func NewHangFault() Fault {
	return Fault{hang: true}
}

type queuedFault struct {
	path  string
	fault Fault
}

type reply struct {
	status int
	body   []byte
}

type deletion struct {
	id   string
	time time.Time
}

// Server is a fake QuickBooks Online API. Its methods are safe for concurrent use.
type Server struct {
	// URL is the server's base URL, as passed to quickbooks.NewClientWithEndpoint.
	URL string

	mu       sync.Mutex
	nextID   int
	entities map[string][]map[string]any // by entity name, in creation order
	deleted  map[string][]deletion
	requests []Request
	replies  map[string]reply // successful creates, by requestid
	faults   []queuedFault
	closing  chan struct{}
}

// NewServer starts a server, closed when the test ends, and returns it with a
// client pointed at it. The fake company starts with a CompanyInfo and a
// Preferences whose home currency is USD, and nothing else.
func NewServer(t testing.TB) (*Server, *quickbooks.Client) {
	t.Helper()

	s := &Server{
		nextID:   1,
		entities: map[string][]map[string]any{},
		deleted:  map[string][]deletion{},
		replies:  map[string]reply{},
		closing:  make(chan struct{}),
	}
	srv := httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	t.Cleanup(func() {
		close(s.closing)
		srv.Close()
	})
	s.URL = srv.URL

	s.Add("CompanyInfo", map[string]any{"CompanyName": "qbotest company", "Country": "US"})
	s.Add("Preferences", map[string]any{"CurrencyPrefs": map[string]any{
		"HomeCurrency": map[string]string{"value": "USD"}, "MultiCurrencyEnabled": false,
	}})

	client, err := quickbooks.NewClientWithEndpoint(quickbooks.EndpointURL(srv.URL), Realm, "", nil)
	if err != nil {
		t.Fatalf("qbotest: %v", err)
	}
	client.Client = srv.Client()

	return s, client
}

// Add stores an entity, such as an Account the code under test will look up,
// and returns its Id. entity is the QuickBooks name ("Account"); v is anything
// that encodes to the entity's JSON, such as a quickbooks.Account. An Id in v
// is kept; otherwise the next free one is used.
func (s *Server) Add(entity string, v any) string {
	name, ok := entityNames[strings.ToLower(entity)]
	if !ok {
		panic("qbotest: unknown entity " + entity)
	}

	obj, err := convertToObject(v)
	if err != nil {
		panic("qbotest: " + err.Error())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store(name, obj)["Id"].(string)
}

// FailNext makes the next request to path fail with f. path is relative to
// /v3/company/{realm}/ ("journalentry", "query"); "" matches any request.
// Queued faults are used in the order they were queued.
func (s *Server) FailNext(path string, f Fault) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.faults = append(s.faults, queuedFault{path: path, fault: f})
}

// ListRequests returns every request the server has received, in order.
func (s *Server) ListRequests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]Request(nil), s.requests...)
}

// ListRequestsTo returns the requests to path ("payment", "query"), in order.
func (s *Server) ListRequestsTo(path string) []Request {
	var out []Request
	for _, r := range s.ListRequests() {
		if r.Path == path {
			out = append(out, r)
		}
	}

	return out
}

// ListEntities returns the stored entities of a type ("JournalEntry") as JSON, in
// creation order, without deleted ones.
func (s *Server) ListEntities(entity string) []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []json.RawMessage
	for _, obj := range s.entities[entityNames[strings.ToLower(entity)]] {
		b, _ := json.Marshal(obj)
		out = append(out, b)
	}

	return out
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	prefix := "/v3/company/" + Realm + "/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	req := Request{Method: r.Method, Path: strings.TrimPrefix(r.URL.Path, prefix), Query: r.URL.Query(), RequestID: r.URL.Query().Get("requestid"), Body: body}

	s.mu.Lock()
	index := len(s.requests)
	s.requests = append(s.requests, req)

	if f, ok := s.takeFault(req.Path); ok {
		if f.hang {
			s.mu.Unlock()
			select {
			case <-r.Context().Done():
			case <-s.closing:
			}
			return
		}

		s.requests[index].Status = f.status
		s.mu.Unlock()
		writeReply(w, reply{status: f.status, body: []byte(f.body)})
		return
	}

	rep := s.handle(req)
	s.requests[index].Status = rep.status
	s.mu.Unlock()

	writeReply(w, rep)
}

func (s *Server) takeFault(path string) (Fault, bool) {
	for i, q := range s.faults {
		if q.path == "" || q.path == path {
			s.faults = append(s.faults[:i], s.faults[i+1:]...)
			return q.fault, true
		}
	}

	return Fault{}, false
}

func writeReply(w http.ResponseWriter, rep reply) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(rep.status)
	_, _ = w.Write(rep.body)
}

// handle answers a request; s.mu is held.
func (s *Server) handle(req Request) reply {
	segments := strings.Split(req.Path, "/")

	switch {
	case req.Method == http.MethodGet && req.Path == "query":
		return s.query(getFirst(req.Query["query"]))
	case req.Method == http.MethodGet && req.Path == "cdc":
		return s.listChanges(getFirst(req.Query["entities"]), getFirst(req.Query["changedSince"]))
	case req.Method == http.MethodGet && len(segments) <= 2:
		return s.read(segments)
	case req.Method == http.MethodPost && len(segments) == 1:
		requestID := getFirst(req.Query["requestid"])
		if prev, ok := s.replies[requestID]; ok && requestID != "" {
			return prev
		}

		rep := s.write(segments[0], getFirst(req.Query["operation"]), getFirst(req.Query["include"]), req.Body)
		if requestID != "" && rep.status == http.StatusOK {
			s.replies[requestID] = rep
		}

		return rep
	}

	return replyWithFault(NewValidationFault("4000", "Unsupported request", "qbotest doesn't support "+req.Method+" "+req.Path))
}

func (s *Server) read(segments []string) reply {
	name, ok := entityNames[segments[0]]
	if !ok {
		return replyUnknownEntity(segments[0])
	}

	// CompanyInfo is read by realm, and Preferences has no Id in its path.
	if name == "CompanyInfo" || name == "Preferences" {
		if objs := s.entities[name]; len(objs) > 0 {
			return replyWithEntity(name, objs[0])
		}
	}

	if len(segments) == 2 {
		if obj := s.find(name, segments[1]); obj != nil {
			return replyWithEntity(name, obj)
		}
	}

	return replyObjectNotFound()
}

func (s *Server) write(path, operation, include string, body []byte) reply {
	name, ok := entityNames[path]
	if !ok {
		return replyUnknownEntity(path)
	}

	obj, err := decodeObject(body)
	if err != nil {
		return replyWithFault(NewValidationFault("2010", "Request has invalid or unsupported property", err.Error()))
	}

	id, _ := obj["Id"].(string)
	if id == "" {
		if operation != "" {
			return replyWithFault(NewValidationFault("2020", "Required param missing, need to supply the required value for the API",
				"Required parameter Id is missing in the request"))
		}

		return replyWithEntity(name, s.store(name, obj))
	}

	existing := s.find(name, id)
	if existing == nil {
		return replyObjectNotFound()
	}

	switch {
	case operation == "delete":
		s.remove(name, id)
		return replyWithEntity(name, map[string]any{"domain": "QBO", "status": "Deleted", "Id": id})
	case operation == "void" || include == "void":
		existing["TotalAmt"] = json.Number("0")
		existing["PrivateNote"] = "Voided"
	case operation == "" || operation == "update":
		if sparse, _ := obj["sparse"].(bool); !sparse {
			for k := range existing {
				if k != "Id" && k != "MetaData" {
					delete(existing, k)
				}
			}
		}
		for k, v := range obj {
			if k != "sparse" && k != "MetaData" {
				existing[k] = v
			}
		}
	default:
		return replyWithFault(NewValidationFault("4000", "Unsupported operation", "qbotest doesn't support operation "+operation))
	}

	syncToken, _ := strconv.Atoi(fmt.Sprint(existing["SyncToken"]))
	existing["SyncToken"] = strconv.Itoa(syncToken + 1)
	existing["domain"] = "QBO"
	existing["sparse"] = false
	setMetaData(existing, false)

	return replyWithEntity(name, existing)
}

var (
	queryPattern = regexp.MustCompile(`(?is)^\s*SELECT\s+(COUNT\(\*\)|\*|[\w\s,.]+?)\s+FROM\s+(\w+)` +
		`(?:\s+WHERE\s+(.+?))?(?:\s+ORDER\s*BY\s+[\w.]+(?:\s+(?:ASC|DESC))?)?` +
		`(?:\s+STARTPOSITION\s+(\d+))?(?:\s+MAXRESULTS\s+(\d+))?\s*$`)
	equalsPattern = regexp.MustCompile(`(?is)^([\w.]+)\s*=\s*(?:'((?:[^']|'')*)'|(\w+))$`)
	inPattern     = regexp.MustCompile(`(?is)^([\w.]+)\s+IN\s*\((.*)\)$`)
	andPattern    = regexp.MustCompile(`(?i)\s+AND\s+`)
)

// query answers a QuickBooks query. It supports COUNT(*) and SELECT, a WHERE
// of "=" and "IN" conditions joined by AND, STARTPOSITION and MAXRESULTS.
// Results are in Id order. As in QuickBooks, inactive entities are left out
// unless the WHERE names Active.
func (s *Server) query(q string) reply {
	m := queryPattern.FindStringSubmatch(q)
	if m == nil {
		return replyQueryError(q)
	}

	name, ok := entityNames[strings.ToLower(m[2])]
	if !ok {
		return replyQueryError(q)
	}

	filters, mentionsActive, ok := parseWhere(m[3])
	if !ok {
		return replyQueryError(q)
	}

	var matched []map[string]any
	for _, obj := range s.entities[name] {
		if !mentionsActive && obj["Active"] == false {
			continue
		}
		if matches(obj, filters) {
			matched = append(matched, obj)
		}
	}

	if strings.EqualFold(m[1], "COUNT(*)") {
		return replyOK(map[string]any{"QueryResponse": map[string]any{"totalCount": len(matched)}, "time": formatNow()})
	}

	start, maxResults := 1, 100
	if m[4] != "" {
		start, _ = strconv.Atoi(m[4])
	}
	if m[5] != "" {
		maxResults, _ = strconv.Atoi(m[5])
	}

	page := []map[string]any{}
	if start >= 1 && start <= len(matched) {
		page = matched[start-1 : min(len(matched), start-1+maxResults)]
	}

	resp := map[string]any{}
	if len(page) > 0 {
		resp[name] = page
		resp["startPosition"] = start
		resp["maxResults"] = len(page)
	}

	return replyOK(map[string]any{"QueryResponse": resp, "time": formatNow()})
}

type filter struct {
	field  string
	values []string
}

func parseWhere(where string) (filters []filter, mentionsActive bool, ok bool) {
	if where == "" {
		return nil, false, true
	}

	for _, cond := range andPattern.Split(strings.TrimSpace(where), -1) {
		var f filter
		if m := equalsPattern.FindStringSubmatch(cond); m != nil {
			f = filter{field: m[1], values: []string{strings.ReplaceAll(m[2], "''", "'") + m[3]}}
		} else if m := inPattern.FindStringSubmatch(cond); m != nil {
			f.field = m[1]
			for _, v := range strings.Split(m[2], ",") {
				v = strings.TrimSpace(v)
				f.values = append(f.values, strings.ReplaceAll(strings.Trim(v, "'"), "''", "'"))
			}
		} else {
			return nil, false, false
		}

		if strings.EqualFold(f.field, "Active") {
			mentionsActive = true
		}
		filters = append(filters, f)
	}

	return filters, mentionsActive, true
}

func matches(obj map[string]any, filters []filter) bool {
	for _, f := range filters {
		var v any = obj
		for _, part := range strings.Split(f.field, ".") {
			m, _ := v.(map[string]any)
			v = lookup(m, part)
		}

		got := fmt.Sprint(v)
		if v == nil && strings.EqualFold(f.field, "Active") {
			got = "true" // an entity without Active is active
		}

		found := false
		for _, want := range f.values {
			if strings.EqualFold(got, want) {
				found = true
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// lookup finds a field case-insensitively, as QuickBooks matches query field names.
func lookup(m map[string]any, field string) any {
	for k, v := range m {
		if strings.EqualFold(k, field) {
			return v
		}
	}

	return nil
}

// listChanges answers a change data capture request: entities updated since
// changedSince, and those deleted since, with "status": "Deleted".
func (s *Server) listChanges(entities, changedSince string) reply {
	since, err := time.Parse(time.RFC3339, changedSince)
	if err != nil {
		return replyWithFault(NewValidationFault("2030", "Invalid parameter", "changedSince: "+err.Error()))
	}

	var responses []map[string]any
	for _, entity := range strings.Split(entities, ",") {
		name, ok := entityNames[strings.ToLower(strings.TrimSpace(entity))]
		if !ok {
			return replyUnknownEntity(entity)
		}

		var changed []map[string]any
		for _, obj := range s.entities[name] {
			if getLastUpdated(obj).After(since) || getLastUpdated(obj).Equal(since) {
				changed = append(changed, obj)
			}
		}
		for _, d := range s.deleted[name] {
			if !d.time.Before(since) {
				changed = append(changed, map[string]any{"domain": "QBO", "status": "Deleted", "Id": d.id,
					"MetaData": map[string]any{"LastUpdatedTime": d.time.Format(time.RFC3339)}})
			}
		}

		resp := map[string]any{"startPosition": 1, "maxResults": len(changed), "totalCount": len(changed)}
		if len(changed) > 0 {
			resp[name] = changed
		}
		responses = append(responses, resp)
	}

	return replyOK(map[string]any{"CDCResponse": []map[string]any{{"QueryResponse": responses}}, "time": formatNow()})
}

func getLastUpdated(obj map[string]any) time.Time {
	meta, _ := obj["MetaData"].(map[string]any)
	t, _ := time.Parse(time.RFC3339, fmt.Sprint(meta["LastUpdatedTime"]))

	return t
}

// store adds obj under name, filling in Id, SyncToken and MetaData; s.mu is held.
func (s *Server) store(name string, obj map[string]any) map[string]any {
	if id, _ := obj["Id"].(string); id != "" {
		if n, err := strconv.Atoi(id); err == nil && n >= s.nextID {
			s.nextID = n + 1
		}
	} else {
		obj["Id"] = strconv.Itoa(s.nextID)
		s.nextID++
	}

	delete(obj, "sparse")
	if _, ok := obj["SyncToken"]; !ok {
		obj["SyncToken"] = "0"
	}
	obj["domain"] = "QBO"
	obj["sparse"] = false
	setMetaData(obj, true)

	s.entities[name] = append(s.entities[name], obj)
	sort.SliceStable(s.entities[name], func(i, j int) bool { return isIDLess(s.entities[name][i], s.entities[name][j]) })

	return obj
}

func (s *Server) find(name, id string) map[string]any {
	for _, obj := range s.entities[name] {
		if obj["Id"] == id {
			return obj
		}
	}

	return nil
}

func (s *Server) remove(name, id string) {
	objs := s.entities[name]
	for i, obj := range objs {
		if obj["Id"] == id {
			s.entities[name] = append(objs[:i], objs[i+1:]...)
			s.deleted[name] = append(s.deleted[name], deletion{id: id, time: time.Now().Truncate(time.Second)})
			return
		}
	}
}

func setMetaData(obj map[string]any, create bool) {
	now := time.Now().Format(time.RFC3339)
	meta, _ := obj["MetaData"].(map[string]any)
	if meta == nil || create {
		meta = map[string]any{"CreateTime": now}
	}
	meta["LastUpdatedTime"] = now
	obj["MetaData"] = meta
}

func isIDLess(a, b map[string]any) bool {
	x, errX := strconv.Atoi(fmt.Sprint(a["Id"]))
	y, errY := strconv.Atoi(fmt.Sprint(b["Id"]))
	if errX != nil || errY != nil {
		return fmt.Sprint(a["Id"]) < fmt.Sprint(b["Id"])
	}

	return x < y
}

func convertToObject(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return decodeObject(b)
}

// decodeObject decodes a JSON object, keeping numbers as written.
func decodeObject(b []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()

	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("body is not a JSON object")
	}

	return obj, nil
}

func replyWithEntity(name string, obj map[string]any) reply {
	return replyOK(map[string]any{name: obj, "time": formatNow()})
}

func replyOK(v any) reply {
	b, _ := json.Marshal(v)

	return reply{status: http.StatusOK, body: b}
}

func replyWithFault(f Fault) reply {
	return reply{status: f.status, body: []byte(f.body)}
}

func replyObjectNotFound() reply {
	return replyWithFault(NewValidationFault("610", "Object Not Found", "Object Not Found : Something you're trying to use has been made inactive. Check the fields with accounts, customers, items, vendors or employees."))
}

func replyUnknownEntity(path string) reply {
	return replyWithFault(NewValidationFault("4000", "Unsupported entity", "qbotest doesn't know the entity "+path))
}

func replyQueryError(q string) reply {
	return replyWithFault(NewValidationFault("4000", "Error parsing query", "QueryParserError: qbotest can't run the query "+q))
}

func formatNow() string {
	return time.Now().Format("2006-01-02T15:04:05.000-07:00")
}

func getFirst(values []string) string {
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

// entityNames maps lowercase entity names, as used in API paths, to QuickBooks' names.
var entityNames = func() map[string]string {
	names := map[string]string{}
	for _, name := range []string{
		"Account", "Attachable", "Bill", "BillPayment", "Budget", "Class", "CompanyInfo", "CreditMemo",
		"Customer", "CustomerType", "Department", "Deposit", "Employee", "Estimate", "ExchangeRate",
		"Invoice", "Item", "JournalEntry", "Payment", "PaymentMethod", "Preferences", "Purchase",
		"PurchaseOrder", "RefundReceipt", "SalesReceipt", "TaxAgency", "TaxCode", "TaxRate", "Term",
		"TimeActivity", "Transfer", "Vendor", "VendorCredit",
	} {
		names[strings.ToLower(name)] = name
	}

	return names
}()
