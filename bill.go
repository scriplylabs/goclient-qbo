package quickbooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// Bill represents a QuickBooks Bill object as returned by the API.
// Read-only fields (Id, SyncToken, MetaData, TotalAmt, HomeBalance, Balance, RecurDataRef)
// are populated by the service.
type Bill struct {
	ID                      string         `json:"Id,omitempty"`
	SyncToken               string         `json:",omitempty"`
	MetaData                *MetaData      `json:",omitempty"`
	VendorRef               ReferenceType  `json:",omitempty"`
	Line                    []Line
	TxnDate                 *Date          `json:",omitempty"`
	DueDate                 *Date          `json:",omitempty"`
	DocNumber               *string        `json:",omitempty"`
	PrivateNote             *string        `json:",omitempty"`
	APAccountRef            *ReferenceType `json:",omitempty"`
	SalesTermRef            *ReferenceType `json:",omitempty"`
	CurrencyRef             *ReferenceType `json:",omitempty"`
	ExchangeRate            json.Number    `json:",omitempty"`
	DepartmentRef           *ReferenceType `json:",omitempty"`
	TransactionLocationType *string        `json:",omitempty"`
	IncludeInAnnualTPAR     *bool          `json:",omitempty"`
	LinkedTxn               []LinkedTxn    `json:",omitempty"`
	TxnTaxDetail            *TxnTaxDetail  `json:",omitempty"`
	// GlobalTaxCalculation: TaxExcluded, TaxInclusive, NotApplicable
	TotalAmt     json.Number    `json:",omitempty"`
	HomeBalance  json.Number    `json:",omitempty"`
	Balance      json.Number    `json:",omitempty"`
	RecurDataRef *ReferenceType `json:",omitempty"`
}

// BillCreateInput contains the writable fields accepted when creating a Bill.
// VendorRef and Line are required; all other fields are optional.
type BillCreateInput struct {
	VendorRef               ReferenceType  `json:",omitempty"`
	Line                    []Line
	TxnDate                 *Date          `json:",omitempty"`
	DueDate                 *Date          `json:",omitempty"`
	DocNumber               *string        `json:",omitempty"`
	PrivateNote             *string        `json:",omitempty"`
	APAccountRef            *ReferenceType `json:",omitempty"`
	SalesTermRef            *ReferenceType `json:",omitempty"`
	CurrencyRef             *ReferenceType `json:",omitempty"`
	ExchangeRate            json.Number    `json:",omitempty"`
	DepartmentRef           *ReferenceType `json:",omitempty"`
	TransactionLocationType *string        `json:",omitempty"`
	IncludeInAnnualTPAR     *bool          `json:",omitempty"`
	LinkedTxn               []LinkedTxn    `json:",omitempty"`
	TxnTaxDetail            *TxnTaxDetail  `json:",omitempty"`
	// GlobalTaxCalculation: TaxExcluded, TaxInclusive, NotApplicable
}

// CreateBill creates the given Bill on the QuickBooks server, returning
// the resulting Bill object. Given WithRequestID, a create QuickBooks has
// seen before returns the bill it made then instead of a second one.
//
//	bill, err := client.CreateBill(&input, quickbooks.WithRequestID(key))
func (c *Client) CreateBill(input *BillCreateInput, opts ...RequestOption) (*Bill, error) {
	var resp struct {
		Bill Bill
		Time Date
	}

	if err := c.post("bill", input, &resp, buildQueryParameters(opts)); err != nil {
		return nil, err
	}

	return &resp.Bill, nil
}

// DeleteBill deletes the bill.
func (c *Client) DeleteBill(bill *Bill) error {
	if bill.ID == "" || bill.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post("bill", bill, nil, map[string]string{"operation": "delete"})
}

// FindBills gets the full list of Bills in the QuickBooks account.
func (c *Client) FindBills() ([]Bill, error) {
	var resp struct {
		QueryResponse struct {
			Bills         []Bill `json:"Bill"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM Bill", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no bills could be found")
	}

	bills := make([]Bill, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM Bill ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.Bills == nil {
			return nil, errors.New("no bills could be found")
		}

		bills = append(bills, resp.QueryResponse.Bills...)
	}

	return bills, nil
}

// FindBillByID finds the bill by the given id.
func (c *Client) FindBillByID(id string) (*Bill, error) {
	var resp struct {
		Bill Bill
		Time Date
	}

	if err := c.get("bill/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Bill, nil
}

// QueryBills accepts an SQL query and returns all bills found using it.
func (c *Client) QueryBills(query string) ([]Bill, error) {
	var resp struct {
		QueryResponse struct {
			Bills         []Bill `json:"Bill"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.Bills == nil {
		return nil, errors.New("could not find any bills")
	}

	return resp.QueryResponse.Bills, nil
}

// ListBills returns one page of Bills ordered by Id.
// Pass an empty pageToken to start from the beginning.
// The returned nextPageToken is empty when there are no more results.
func (c *Client) ListBills(pageToken string, pageSize int) (*ListResponse[Bill], error) {
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
			Bills         []Bill `json:"Bill"`
			StartPosition int
			MaxResults    int
		}
	}

	query := "SELECT * FROM Bill ORDERBY Id STARTPOSITION " + strconv.Itoa(startPosition) + " MAXRESULTS " + strconv.Itoa(pageSize)
	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	result := &ListResponse[Bill]{Items: resp.QueryResponse.Bills}
	if len(result.Items) == pageSize {
		result.NextPageToken = strconv.Itoa(startPosition + pageSize)
	}

	return result, nil
}

// UpdateBill updates the bill.
func (c *Client) UpdateBill(bill *Bill) (*Bill, error) {
	if bill.ID == "" {
		return nil, errors.New("missing bill id")
	}

	existingBill, err := c.FindBillByID(bill.ID)
	if err != nil {
		return nil, err
	}

	bill.SyncToken = existingBill.SyncToken

	payload := struct {
		*Bill
		Sparse bool `json:"sparse"`
	}{
		Bill:   bill,
		Sparse: true,
	}

	var billData struct {
		Bill Bill
		Time Date
	}

	if err = c.post("bill", payload, &billData, nil); err != nil {
		return nil, err
	}

	return &billData.Bill, err
}
