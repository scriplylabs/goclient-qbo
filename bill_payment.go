package quickbooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// BillPaymentCheckPayment contains details for a check-based bill payment.
type BillPaymentCheckPayment struct {
	BankAccountRef ReferenceType `json:",omitempty"`
	PrintStatus    *string       `json:",omitempty"`
}

// BillPaymentCreditCardPayment contains details for a credit-card-based bill payment.
type BillPaymentCreditCardPayment struct {
	CCAccountRef ReferenceType `json:",omitempty"`
}

// BillPayment represents a QuickBooks BillPayment object as returned by the API.
// Read-only fields (Id, SyncToken, MetaData) are populated by the service.
type BillPayment struct {
	ID                string                        `json:"Id,omitempty"`
	SyncToken         string                        `json:",omitempty"`
	MetaData          *MetaData                     `json:",omitempty"`
	VendorRef         ReferenceType                 `json:",omitempty"`
	PayType           string                        `json:",omitempty"`
	CheckPayment      *BillPaymentCheckPayment      `json:",omitempty"`
	CreditCardPayment *BillPaymentCreditCardPayment  `json:",omitempty"`
	TotalAmt          json.Number                   `json:",omitempty"`
	Line              []PaymentLine                 `json:",omitempty"`
	TxnDate           *Date                         `json:",omitempty"`
	DocNumber         *string                       `json:",omitempty"`
	PrivateNote       *string                       `json:",omitempty"`
	APAccountRef      *ReferenceType                `json:",omitempty"`
	CurrencyRef       *ReferenceType                `json:",omitempty"`
	ExchangeRate      json.Number                   `json:",omitempty"`
	DepartmentRef     *ReferenceType                `json:",omitempty"`
}

// BillPaymentCreateInput contains the writable fields accepted when creating a BillPayment.
// VendorRef, PayType, TotalAmt, and Line are required.
type BillPaymentCreateInput struct {
	VendorRef         ReferenceType                 `json:",omitempty"`
	PayType           string                        `json:",omitempty"`
	CheckPayment      *BillPaymentCheckPayment      `json:",omitempty"`
	CreditCardPayment *BillPaymentCreditCardPayment  `json:",omitempty"`
	TotalAmt          json.Number                   `json:",omitempty"`
	Line              []PaymentLine                 `json:",omitempty"`
	TxnDate           *Date                         `json:",omitempty"`
	DocNumber         *string                       `json:",omitempty"`
	PrivateNote       *string                       `json:",omitempty"`
	APAccountRef      *ReferenceType                `json:",omitempty"`
	CurrencyRef       *ReferenceType                `json:",omitempty"`
	ExchangeRate      json.Number                   `json:",omitempty"`
	DepartmentRef     *ReferenceType                `json:",omitempty"`
}

// CreateBillPayment creates the given BillPayment on the QuickBooks server, returning
// the resulting BillPayment object.
//
// Pass WithRequestID to make retries of the same create safe.
func (c *Client) CreateBillPayment(input *BillPaymentCreateInput, opts ...RequestOption) (*BillPayment, error) {
	var resp struct {
		BillPayment BillPayment
		Time        Date
	}

	if err := c.post("billpayment", input, &resp, queryParameters(opts)); err != nil {
		return nil, err
	}

	return &resp.BillPayment, nil
}

// DeleteBillPayment deletes the bill payment.
func (c *Client) DeleteBillPayment(billPayment *BillPayment) error {
	if billPayment.ID == "" || billPayment.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post("billpayment", billPayment, nil, map[string]string{"operation": "delete"})
}

// FindBillPayments gets the full list of BillPayments in the QuickBooks account.
func (c *Client) FindBillPayments() ([]BillPayment, error) {
	var resp struct {
		QueryResponse struct {
			BillPayments  []BillPayment `json:"BillPayment"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM BillPayment", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no bill payments could be found")
	}

	billPayments := make([]BillPayment, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM BillPayment ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.BillPayments == nil {
			return nil, errors.New("no bill payments could be found")
		}

		billPayments = append(billPayments, resp.QueryResponse.BillPayments...)
	}

	return billPayments, nil
}

// FindBillPaymentByID finds the bill payment by the given id.
func (c *Client) FindBillPaymentByID(id string) (*BillPayment, error) {
	var resp struct {
		BillPayment BillPayment
		Time        Date
	}

	if err := c.get("billpayment/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.BillPayment, nil
}

// QueryBillPayments accepts an SQL query and returns all bill payments found using it.
func (c *Client) QueryBillPayments(query string) ([]BillPayment, error) {
	var resp struct {
		QueryResponse struct {
			BillPayments  []BillPayment `json:"BillPayment"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.BillPayments == nil {
		return nil, errors.New("could not find any bill payments")
	}

	return resp.QueryResponse.BillPayments, nil
}

// ListBillPayments returns one page of BillPayments ordered by Id.
// Pass an empty pageToken to start from the beginning.
// The returned nextPageToken is empty when there are no more results.
func (c *Client) ListBillPayments(pageToken string, pageSize int) (*ListResponse[BillPayment], error) {
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
			BillPayments  []BillPayment `json:"BillPayment"`
			StartPosition int
			MaxResults    int
		}
	}

	query := "SELECT * FROM BillPayment ORDERBY Id STARTPOSITION " + strconv.Itoa(startPosition) + " MAXRESULTS " + strconv.Itoa(pageSize)
	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	result := &ListResponse[BillPayment]{Items: resp.QueryResponse.BillPayments}
	if len(result.Items) == pageSize {
		result.NextPageToken = strconv.Itoa(startPosition + pageSize)
	}

	return result, nil
}

// UpdateBillPayment updates the bill payment.
func (c *Client) UpdateBillPayment(billPayment *BillPayment) (*BillPayment, error) {
	if billPayment.ID == "" {
		return nil, errors.New("missing bill payment id")
	}

	existingBillPayment, err := c.FindBillPaymentByID(billPayment.ID)
	if err != nil {
		return nil, err
	}

	billPayment.SyncToken = existingBillPayment.SyncToken

	payload := struct {
		*BillPayment
		Sparse bool `json:"sparse"`
	}{
		BillPayment: billPayment,
		Sparse:      true,
	}

	var billPaymentData struct {
		BillPayment BillPayment
		Time        Date
	}

	if err = c.post("billpayment", payload, &billPaymentData, nil); err != nil {
		return nil, err
	}

	return &billPaymentData.BillPayment, err
}
