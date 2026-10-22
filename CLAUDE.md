# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run all tests
go test ./...

# Run a single test
go test -run TestAccount .

# Run tests with verbose output
go test -v ./...

# Vet
go vet ./...

# Build (library — confirm it compiles)
go build ./...

# Tests against a real QuickBooks sandbox (needs token.txt; see sandbox_test.go)
go test -tags sandbox -run Sandbox -v .
```

## Architecture

This is a Go client library for the QuickBooks Online (QBO) REST API. The package name is `quickbooks` and the install path is `github.com/scriplylabs/goclient-qbo`.

**Core files:**
- `client.go` — `Client` struct, `NewClient`, `NewClientWithEndpoint` (any base URL, no discovery call), `RequestOption`/`WithRequestID`, and the internal HTTP helpers `req`/`get`/`post`/`query`
- `defs.go` — shared types: `Date`, `DateTime`, `Address`, `ReferenceType`, `MetaData`, `MemoRef`, `TelephoneNumber`, `WebSiteAddress`, constants (`ProductionEndpoint`, `SandboxEndpoint`, `queryPageSize`)
- `errors.go` — `Failure` struct, `parseFailure`
- `token.go` — OAuth2 bearer token; `getHttpClient` wraps a token into an `*http.Client`
- `discovery.go` — fetches OAuth2 endpoints from Intuit's discovery document
- `changed_data_capture_entities.go` — `MaybeDeleted[T]`, `DeletedEntity` generics used by CDC
- `change_data_capture.go` — `GetChangedEntities` using QBO CDC API
- `qbotest/` — exported fake QBO server (`qbotest.NewServer(t)`) for tests of code that uses this library: records requests, stores entities, answers queries and CDC, replays `requestid`, and returns faults queued with `FailNext`
- `sandbox_test.go` — tests against a real sandbox company, behind the `sandbox` build tag; they pin the QBO behaviour qbotest copies, so run them when QBO may have changed

**Per-entity files** (`account.go`, `attachable.go`, `bill.go`, `class.go`, `customer.go`, `invoice.go`, `item.go`, `payment.go`, `vendor.go`, etc.) each contain:
1. A **domain struct** (e.g. `Account`) — represents the full API response, including read-only fields
2. A **create-input struct** (e.g. `AccountCreateInput`) — contains only writable fields accepted on create
3. CRUD methods on `*Client`

## Required Patterns

### Create vs Update interface

**Create** must accept a dedicated `*EntityCreateInput` type — never the domain object:

```go
func (c *Client) CreateAccount(input *AccountCreateInput) (*Account, error)
```

**Update** accepts the domain struct `*Entity`. The method internally fetches the current `SyncToken` and performs a sparse POST:

```go
func (c *Client) UpdateAccount(account *Account) (*Account, error) {
    existingAccount, err := c.FindAccountByID(account.ID)
    account.SyncToken = existingAccount.SyncToken
    payload := struct {
        *Account
        Sparse bool `json:"sparse"`
    }{Account: account, Sparse: true}
    // ...
}
```

### Optional fields must be pointers

Any field the QBO API marks as optional **must** be a pointer type. Required fields (like `Name`, `AccountType` on Account) use value types. Read-only server-populated fields (like `FullyQualifiedName`, `CurrentBalance`, `Balance`) stay as value types on the domain struct but are **omitted** from the create-input struct.

```go
// Optional → pointer
Active          *bool          `json:",omitempty"`
AcctNum         *string        `json:",omitempty"`
ParentRef       *ReferenceType `json:",omitempty"`

// Required on create → value type
Name        string `json:",omitempty"`
AccountType string `json:",omitempty"`
```

### JSON tags

- Match QBO API field names exactly (PascalCase): `json:"Id,omitempty"`, `json:"FileAccessUri,omitempty"`
- When the Go field name would differ from the JSON key (e.g. `ID` vs `Id`), use an explicit tag: `json:"Id,omitempty"`
- Use `json.Number` for all monetary/numeric amounts (not `float64`) to avoid precision loss

### Pagination

All `FindAll` methods follow this pattern — first count, then paginate at `queryPageSize` (1000):

```go
if err := c.query("SELECT COUNT(*) FROM Account", &resp); err != nil { ... }
for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
    query := "SELECT * FROM Account ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)
    // ...
}
```

### Named returns with deferred body close

Functions that own an `*http.Response` body use named returns so the deferred close error is captured:

```go
func (c *Client) DownloadAttachable(id string) (s string, e error) {
    // ...
    defer func() { e = resp.Body.Close() }()
}
```

### Line encoding

`Line` keeps its detail fields value-typed (callers read them back), and `Line.MarshalJSON` writes only the detail its `DetailType` names, plus `GroupLineDetail` when set. A new detail field on `Line` needs a case there too.

### Optional request parameters

Methods that take an optional query parameter accept `opts ...RequestOption` and pass `buildQueryParameters(opts)` to `post`/`get`; `WithRequestID` is the first. Creates that post money (`CreatePayment`, `CreateBillPayment`, `CreateJournalEntry`) take it today.

### Faults

QBO sends two fault shapes, and `Failure` must decode both: validation faults use `Fault`/`Error` with a string `time`; authentication faults (401, and 403 for scope) use lowercase `fault`/`error` with `time` in epoch milliseconds. CDC and delete responses report `"status": "Deleted"` with a capital D.

### Testing

Unit tests make no live calls: use `NewTestClient` (package-internal) or `qbotest`. Live checks go in `sandbox_test.go` under the `sandbox` tag, create only what they delete, and read credentials from the gitignored `token.txt`. When QBO behaviour changes, update the sandbox test, then `qbotest` to match.

### Domain struct documentation

Comment read-only fields on domain structs so callers know not to set them:

```go
// Account represents a QuickBooks Account object as returned by the API.
// Read-only fields (Id, SyncToken, MetaData, FullyQualifiedName, ...) are populated by the service.
type Account struct { ... }
```

## API Reference

The XSD files in `schema/` (Intuit's official schema, copied from their Java SDK; see `schema/README.md`) are the authoritative reference for entity types, field names, field types, and which fields exist. Entity types are in `schema/Finance.xsd`. Cross-check all struct definitions against it.

- Elements with `minOccurs="0"` are optional → pointer types. Elements without it are required → value types.
- The XSD uses the same types for create, update and read, so it doesn't say which fields are required on create. Use Intuit's per-entity API docs for that.
- `xs:decimal` maps to `json.Number`, `xs:date` maps to `Date` (written as `2006-01-02`), and `xs:dateTime` maps to `DateTime` (written as RFC 3339). Using `Date` for a date and time drops the time on write.

`QuickBooks Online API Collections.postman_collection.json` is a set of example requests, not a complete spec. It's useful for request shapes, but a field or type missing from it doesn't mean it's invalid.
