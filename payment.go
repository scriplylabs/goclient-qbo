package quickbooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// Payment represents a QuickBooks Payment object as returned by the API.
// Read-only fields (Id, SyncToken, MetaData, UnappliedAmt) are populated by the service.
type Payment struct {
	ID                  string         `json:"Id,omitempty"`
	SyncToken           string         `json:",omitempty"`
	MetaData            *MetaData      `json:",omitempty"`
	CustomerRef         ReferenceType  `json:",omitempty"`
	TotalAmt            json.Number    `json:",omitempty"`
	UnappliedAmt        json.Number    `json:",omitempty"`
	TxnDate             *Date          `json:",omitempty"`
	DepositToAccountRef *ReferenceType `json:",omitempty"`
	ProcessPayment      *bool          `json:",omitempty"`
	Line                []PaymentLine  `json:",omitempty"`
}

// PaymentLine represents a line item within a Payment.
type PaymentLine struct {
	Amount    json.Number `json:",omitempty"`
	LinkedTxn []LinkedTxn `json:",omitempty"`
}

// PaymentCreateInput contains the writable fields accepted when creating a Payment.
// CustomerRef and TotalAmt are required; all other fields are optional.
type PaymentCreateInput struct {
	CustomerRef         ReferenceType  `json:",omitempty"`
	TotalAmt            json.Number    `json:",omitempty"`
	TxnDate             *Date          `json:",omitempty"`
	DepositToAccountRef *ReferenceType `json:",omitempty"`
	ProcessPayment      *bool          `json:",omitempty"`
	Line                []PaymentLine  `json:",omitempty"`
}

// CreatePayment creates the given payment within QuickBooks.
//
// Pass WithRequestID to make retries of the same create safe.
func (c *Client) CreatePayment(input *PaymentCreateInput, opts ...RequestOption) (*Payment, error) {
	var resp struct {
		Payment Payment
		Time    Date
	}

	if err := c.post("payment", input, &resp, buildQueryParameters(opts)); err != nil {
		return nil, err
	}

	return &resp.Payment, nil
}

// DeletePayment deletes the given payment from QuickBooks.
func (c *Client) DeletePayment(payment *Payment) error {
	if payment.ID == "" || payment.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post("payment", payment, nil, map[string]string{"operation": "delete"})
}

// FindPayments gets the full list of Payments in the QuickBooks account.
func (c *Client) FindPayments() ([]Payment, error) {
	var resp struct {
		QueryResponse struct {
			Payments      []Payment `json:"Payment"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM Payment", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no payments could be found")
	}

	payments := make([]Payment, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM Payment ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.Payments == nil {
			return nil, errors.New("no payments could be found")
		}

		payments = append(payments, resp.QueryResponse.Payments...)
	}

	return payments, nil
}

// FindPaymentByID returns a payment with a given Id.
func (c *Client) FindPaymentByID(id string) (*Payment, error) {
	var resp struct {
		Payment Payment
		Time    Date
	}

	if err := c.get("payment/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Payment, nil
}

// QueryPayments accepts a SQL query and returns all payments found using it.
func (c *Client) QueryPayments(query string) ([]Payment, error) {
	var resp struct {
		QueryResponse struct {
			Payments      []Payment `json:"Payment"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.Payments == nil {
		return nil, errors.New("could not find any payments")
	}

	return resp.QueryResponse.Payments, nil
}

// ListPayments returns one page of Payments ordered by Id.
// Pass an empty pageToken to start from the beginning.
// The returned nextPageToken is empty when there are no more results.
func (c *Client) ListPayments(pageToken string, pageSize int) (*ListResponse[Payment], error) {
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
			Payments      []Payment `json:"Payment"`
			StartPosition int
			MaxResults    int
		}
	}

	query := "SELECT * FROM Payment ORDERBY Id STARTPOSITION " + strconv.Itoa(startPosition) + " MAXRESULTS " + strconv.Itoa(pageSize)
	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	result := &ListResponse[Payment]{Items: resp.QueryResponse.Payments}
	if len(result.Items) == pageSize {
		result.NextPageToken = strconv.Itoa(startPosition + pageSize)
	}

	return result, nil
}

// UpdatePayment updates the given payment in QuickBooks.
func (c *Client) UpdatePayment(payment *Payment) (*Payment, error) {
	if payment.ID == "" {
		return nil, errors.New("missing payment id")
	}

	existingPayment, err := c.FindPaymentByID(payment.ID)
	if err != nil {
		return nil, err
	}

	payment.SyncToken = existingPayment.SyncToken

	payload := struct {
		*Payment
		Sparse bool `json:"sparse"`
	}{
		Payment: payment,
		Sparse:  true,
	}

	var paymentData struct {
		Payment Payment
		Time    Date
	}

	if err = c.post("payment", payload, &paymentData, nil); err != nil {
		return nil, err
	}

	return &paymentData.Payment, err
}

// VoidPayment voids the given payment in QuickBooks.
func (c *Client) VoidPayment(payment *Payment) error {
	if payment.ID == "" {
		return errors.New("missing payment id")
	}

	existingPayment, err := c.FindPaymentByID(payment.ID)
	if err != nil {
		return err
	}

	payment.SyncToken = existingPayment.SyncToken

	return c.post("payment", payment, nil, map[string]string{"operation": "update", "include": "void"})
}
