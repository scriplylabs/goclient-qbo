package quickbooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// Invoice represents a QuickBooks Invoice object as returned by the API.
// Read-only fields (Id, SyncToken, MetaData, TotalAmt, HomeAmtTotal, HomeBalance,
// Balance, TxnSource, LinkedTxn) are populated by the service.
type Invoice struct {
	ID            string        `json:"Id,omitempty"`
	SyncToken     string        `json:",omitempty"`
	MetaData      *MetaData     `json:",omitempty"`
	CustomField   []CustomField `json:",omitempty"`
	DocNumber     *string       `json:",omitempty"`
	TxnDate       *Date         `json:",omitempty"`
	DepartmentRef *ReferenceType `json:",omitempty"`
	PrivateNote   *string       `json:",omitempty"`
	LinkedTxn     []LinkedTxn   `json:"LinkedTxn"`
	Line          []Line
	TxnTaxDetail  *TxnTaxDetail  `json:",omitempty"`
	CustomerRef   ReferenceType
	CustomerMemo  *MemoRef       `json:",omitempty"`
	BillAddr      *Address       `json:",omitempty"`
	ShipAddr      *Address       `json:",omitempty"`
	ClassRef      *ReferenceType `json:",omitempty"`
	SalesTermRef  *ReferenceType `json:",omitempty"`
	DueDate       *Date          `json:",omitempty"`
	ShipMethodRef *ReferenceType `json:",omitempty"`
	ShipDate      *Date          `json:",omitempty"`
	TrackingNum   *string        `json:",omitempty"`
	TotalAmt      json.Number    `json:",omitempty"`
	CurrencyRef   *ReferenceType `json:",omitempty"`
	ExchangeRate  json.Number    `json:",omitempty"`
	HomeAmtTotal  json.Number    `json:",omitempty"`
	HomeBalance   json.Number    `json:",omitempty"`
	ApplyTaxAfterDiscount        *bool          `json:",omitempty"`
	PrintStatus                  *string        `json:",omitempty"`
	EmailStatus                  *string        `json:",omitempty"`
	BillEmail                    *EmailAddress  `json:",omitempty"`
	BillEmailCC                  *EmailAddress  `json:"BillEmailCc,omitempty"`
	BillEmailBCC                 *EmailAddress  `json:"BillEmailBcc,omitempty"`
	DeliveryInfo                 *DeliveryInfo  `json:",omitempty"`
	Balance                      json.Number    `json:",omitempty"`
	TxnSource                    *string        `json:",omitempty"`
	AllowOnlineCreditCardPayment *bool          `json:",omitempty"`
	AllowOnlineACHPayment        *bool          `json:",omitempty"`
	Deposit                      json.Number    `json:",omitempty"`
	DepositToAccountRef          *ReferenceType `json:",omitempty"`
}

// InvoiceCreateInput contains the writable fields accepted when creating an Invoice.
// CustomerRef and Line are required; all other fields are optional.
type InvoiceCreateInput struct {
	CustomerRef   ReferenceType
	Line          []Line
	DocNumber     *string        `json:",omitempty"`
	TxnDate       *Date          `json:",omitempty"`
	DepartmentRef *ReferenceType `json:",omitempty"`
	PrivateNote   *string        `json:",omitempty"`
	TxnTaxDetail  *TxnTaxDetail  `json:",omitempty"`
	CustomerMemo  *MemoRef       `json:",omitempty"`
	BillAddr      *Address       `json:",omitempty"`
	ShipAddr      *Address       `json:",omitempty"`
	ClassRef      *ReferenceType `json:",omitempty"`
	SalesTermRef  *ReferenceType `json:",omitempty"`
	DueDate       *Date          `json:",omitempty"`
	ShipMethodRef *ReferenceType `json:",omitempty"`
	ShipDate      *Date          `json:",omitempty"`
	TrackingNum   *string        `json:",omitempty"`
	CurrencyRef   *ReferenceType `json:",omitempty"`
	ExchangeRate  json.Number    `json:",omitempty"`
	ApplyTaxAfterDiscount        *bool         `json:",omitempty"`
	PrintStatus                  *string       `json:",omitempty"`
	EmailStatus                  *string       `json:",omitempty"`
	BillEmail                    *EmailAddress `json:",omitempty"`
	BillEmailCC                  *EmailAddress `json:"BillEmailCc,omitempty"`
	BillEmailBCC                 *EmailAddress `json:"BillEmailBcc,omitempty"`
	AllowOnlineCreditCardPayment *bool         `json:",omitempty"`
	AllowOnlineACHPayment        *bool         `json:",omitempty"`
	Deposit                      json.Number   `json:",omitempty"`
	DepositToAccountRef          *ReferenceType `json:",omitempty"`
	CustomField                  []CustomField  `json:",omitempty"`
}

type DeliveryInfo struct {
	DeliveryType string
	DeliveryTime DateTime
}

// LinkedTxn links a line to another transaction. TxnLineID names the linked
// transaction's line; a deposit line banking a payment needs it ("0" for a
// Payment's own line).
type LinkedTxn struct {
	TxnID     string  `json:"TxnId"`
	TxnType   string  `json:"TxnType"`
	TxnLineID *string `json:"TxnLineId,omitempty"`
}

type TxnTaxDetail struct {
	TxnTaxCodeRef ReferenceType `json:",omitempty"`
	TotalTax      json.Number   `json:",omitempty"`
	TaxLine       []Line        `json:",omitempty"`
}

type AccountBasedExpenseLineDetail struct {
	AccountRef     ReferenceType
	CustomerRef    *ReferenceType `json:",omitempty"`
	ClassRef       *ReferenceType `json:",omitempty"`
	DepartmentRef  *ReferenceType `json:",omitempty"`
	TaxCodeRef     *ReferenceType `json:",omitempty"`
	TaxAmount      json.Number    `json:",omitempty"`
	BillableStatus *string        `json:",omitempty"`
}

// JournalEntryLineDetail holds the detail for a JournalEntry line.
type JournalEntryLineDetail struct {
	PostingType     string         // "Debit" or "Credit"
	AccountRef      ReferenceType
	ClassRef        *ReferenceType `json:",omitempty"`
	DepartmentRef   *ReferenceType `json:",omitempty"`
	TaxCodeRef      *ReferenceType `json:",omitempty"`
	Entity          *EntityTypeRef `json:",omitempty"`
	BillableStatus  *string        `json:",omitempty"`
	TaxApplicableOn *string        `json:",omitempty"`
}

// EntityTypeRef is used in JournalEntryLineDetail to reference an entity (customer, vendor, etc.).
type EntityTypeRef struct {
	Type      string        `json:",omitempty"`
	EntityRef ReferenceType `json:",omitempty"`
}

// ItemBasedExpenseLineDetail holds the detail for an item-based expense line (used in PurchaseOrder).
type ItemBasedExpenseLineDetail struct {
	ItemRef         ReferenceType
	ClassRef        *ReferenceType `json:",omitempty"`
	UnitPrice       json.Number    `json:",omitempty"`
	Qty             json.Number    `json:",omitempty"`
	TaxCodeRef      *ReferenceType `json:",omitempty"`
	CustomerRef     *ReferenceType `json:",omitempty"`
	BillableStatus  *string        `json:",omitempty"`
	TaxInclusiveAmt json.Number    `json:",omitempty"`
}

type Line struct {
	ID                            string                     `json:"Id,omitempty"`
	LineNum                       int                        `json:",omitempty"`
	Description                   string                     `json:",omitempty"`
	Amount                        json.Number
	DetailType                    string
	AccountBasedExpenseLineDetail AccountBasedExpenseLineDetail `json:",omitempty"`
	SalesItemLineDetail           SalesItemLineDetail           `json:",omitempty"`
	DiscountLineDetail            DiscountLineDetail            `json:",omitempty"`
	TaxLineDetail                 TaxLineDetail                `json:",omitempty"`
	JournalEntryLineDetail        JournalEntryLineDetail       `json:",omitempty"`
	ItemBasedExpenseLineDetail    ItemBasedExpenseLineDetail   `json:",omitempty"`
	GroupLineDetail               *GroupLineDetail             `json:",omitempty"`
}

// MarshalJSON writes only the line detail that DetailType names, plus
// GroupLineDetail when it is set. The detail fields are structs, which
// omitempty never omits, so without this every line would carry every kind of
// detail. A DetailType with no matching field (DescriptionOnly,
// SubTotalLineDetail) writes no detail.
func (l Line) MarshalJSON() ([]byte, error) {
	out := struct {
		ID                            string                         `json:"Id,omitempty"`
		LineNum                       int                            `json:",omitempty"`
		Description                   string                         `json:",omitempty"`
		Amount                        json.Number
		DetailType                    string
		AccountBasedExpenseLineDetail *AccountBasedExpenseLineDetail `json:",omitempty"`
		SalesItemLineDetail           *SalesItemLineDetail           `json:",omitempty"`
		DiscountLineDetail            *DiscountLineDetail            `json:",omitempty"`
		TaxLineDetail                 *TaxLineDetail                 `json:",omitempty"`
		JournalEntryLineDetail        *JournalEntryLineDetail        `json:",omitempty"`
		ItemBasedExpenseLineDetail    *ItemBasedExpenseLineDetail    `json:",omitempty"`
		GroupLineDetail               *GroupLineDetail               `json:",omitempty"`
	}{
		ID:              l.ID,
		LineNum:         l.LineNum,
		Description:     l.Description,
		Amount:          l.Amount,
		DetailType:      l.DetailType,
		GroupLineDetail: l.GroupLineDetail,
	}

	switch l.DetailType {
	case "AccountBasedExpenseLineDetail":
		out.AccountBasedExpenseLineDetail = &l.AccountBasedExpenseLineDetail
	case "SalesItemLineDetail":
		out.SalesItemLineDetail = &l.SalesItemLineDetail
	case "DiscountLineDetail":
		out.DiscountLineDetail = &l.DiscountLineDetail
	case "TaxLineDetail":
		out.TaxLineDetail = &l.TaxLineDetail
	case "JournalEntryLineDetail":
		out.JournalEntryLineDetail = &l.JournalEntryLineDetail
	case "ItemBasedExpenseLineDetail":
		out.ItemBasedExpenseLineDetail = &l.ItemBasedExpenseLineDetail
	}

	return json.Marshal(out)
}

// GroupLineDetail holds a bundle (group item) line. The bundle's component
// sales lines are nested in Line.
type GroupLineDetail struct {
	GroupItemRef ReferenceType
	Quantity     json.Number `json:",omitempty"`
	UOMRef       *UOMRef     `json:",omitempty"`
	ServiceDate  *Date       `json:",omitempty"`
	Line         []Line      `json:",omitempty"`
}

// UOMRef references a unit of measure.
type UOMRef struct {
	Unit      string
	UOMSetRef *ReferenceType `json:",omitempty"`
}

// TaxLineDetail ...
type TaxLineDetail struct {
	PercentBased     *bool       `json:",omitempty"`
	NetAmountTaxable json.Number `json:",omitempty"`
	TaxPercent       json.Number `json:",omitempty"`
	TaxRateRef       ReferenceType
}

// SalesItemLineDetail ...
type SalesItemLineDetail struct {
	ItemRef        *ReferenceType `json:",omitempty"`
	ClassRef       *ReferenceType `json:",omitempty"`
	UnitPrice      json.Number    `json:",omitempty"`
	Qty            json.Number    `json:",omitempty"`
	ItemAccountRef *ReferenceType `json:",omitempty"`
	TaxCodeRef     *ReferenceType `json:",omitempty"`
	ServiceDate    *Date          `json:",omitempty"`
	TaxInclusiveAmt json.Number   `json:",omitempty"`
	DiscountRate   json.Number    `json:",omitempty"`
	DiscountAmt    json.Number    `json:",omitempty"`
}

// DiscountLineDetail ...
type DiscountLineDetail struct {
	PercentBased    *bool       `json:",omitempty"`
	DiscountPercent json.Number `json:",omitempty"`
}

// CreateInvoice creates the given Invoice on the QuickBooks server, returning
// the resulting Invoice object.
func (c *Client) CreateInvoice(input *InvoiceCreateInput) (*Invoice, error) {
	var resp struct {
		Invoice Invoice
		Time    Date
	}

	if err := c.post("invoice", input, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Invoice, nil
}

// DeleteInvoice deletes the invoice
//
// If the invoice was already deleted, QuickBooks returns 400 :(
// The response looks like this:
// {"Fault":{"Error":[{"Message":"Object Not Found","Detail":"Object Not Found : Something you're trying to use has been made inactive. Check the fields with accounts, invoices, items, vendors or employees.","code":"610","element":""}],"type":"ValidationFault"},"time":"2018-03-20T20:15:59.571-07:00"}
//
// This is slightly horrifying and not documented in their API. When this
// happens we just return success; the goal of deleting it has been
// accomplished, just not by us.
func (c *Client) DeleteInvoice(invoice *Invoice) error {
	if invoice.ID == "" || invoice.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post("invoice", invoice, nil, map[string]string{"operation": "delete"})
}

// FindInvoices gets the full list of Invoices in the QuickBooks account.
func (c *Client) FindInvoices() ([]Invoice, error) {
	var resp struct {
		QueryResponse struct {
			Invoices      []Invoice `json:"Invoice"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM Invoice", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no invoices could be found")
	}

	invoices := make([]Invoice, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM Invoice ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.Invoices == nil {
			return nil, errors.New("no invoices could be found")
		}

		invoices = append(invoices, resp.QueryResponse.Invoices...)
	}

	return invoices, nil
}

// FindInvoiceByID finds the invoice by the given id
func (c *Client) FindInvoiceByID(id string) (*Invoice, error) {
	var resp struct {
		Invoice Invoice
		Time    Date
	}

	if err := c.get("invoice/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Invoice, nil
}

// QueryInvoices accepts an SQL query and returns all invoices found using it
func (c *Client) QueryInvoices(query string) ([]Invoice, error) {
	var resp struct {
		QueryResponse struct {
			Invoices      []Invoice `json:"Invoice"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.Invoices == nil {
		return nil, errors.New("could not find any invoices")
	}

	return resp.QueryResponse.Invoices, nil
}

// ListInvoices returns one page of Invoices ordered by Id.
// Pass an empty pageToken to start from the beginning.
// The returned nextPageToken is empty when there are no more results.
func (c *Client) ListInvoices(pageToken string, pageSize int) (*ListResponse[Invoice], error) {
	if pageSize <= 0 || pageSize > queryPageSize {
		pageSize = queryPageSize
	}

	startPosition := 1
	if pageToken != "" {
		var err error
		startPosition, err = strconv.Atoi(pageToken)
		if err != nil {
			return nil, fmt.Errorf("invalid page token: %v", err)
		}
	}

	var resp struct {
		QueryResponse struct {
			Invoices      []Invoice `json:"Invoice"`
			StartPosition int
			MaxResults    int
		}
	}

	query := "SELECT * FROM Invoice ORDERBY Id STARTPOSITION " + strconv.Itoa(startPosition) + " MAXRESULTS " + strconv.Itoa(pageSize)
	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	result := &ListResponse[Invoice]{Items: resp.QueryResponse.Invoices}
	if len(result.Items) == pageSize {
		result.NextPageToken = strconv.Itoa(startPosition + pageSize)
	}

	return result, nil
}

// SendInvoice sends the invoice to the Invoice.BillEmail if emailAddress is left empty
func (c *Client) SendInvoice(invoiceId string, emailAddress string) error {
	queryParameters := make(map[string]string)

	if emailAddress != "" {
		queryParameters["sendTo"] = emailAddress
	}

	return c.post("invoice/"+invoiceId+"/send", nil, nil, queryParameters)
}

// UpdateInvoice updates the invoice
func (c *Client) UpdateInvoice(invoice *Invoice) (*Invoice, error) {
	if invoice.ID == "" {
		return nil, errors.New("missing invoice id")
	}

	existingInvoice, err := c.FindInvoiceByID(invoice.ID)
	if err != nil {
		return nil, err
	}

	invoice.SyncToken = existingInvoice.SyncToken

	payload := struct {
		*Invoice
		Sparse bool `json:"sparse"`
	}{
		Invoice: invoice,
		Sparse:  true,
	}

	var invoiceData struct {
		Invoice Invoice
		Time    Date
	}

	if err = c.post("invoice", payload, &invoiceData, nil); err != nil {
		return nil, err
	}

	return &invoiceData.Invoice, err
}

func (c *Client) VoidInvoice(invoice *Invoice) error {
	if invoice.ID == "" {
		return errors.New("missing invoice id")
	}

	existingInvoice, err := c.FindInvoiceByID(invoice.ID)
	if err != nil {
		return err
	}

	invoice.SyncToken = existingInvoice.SyncToken

	return c.post("invoice", invoice, nil, map[string]string{"operation": "void"})
}
