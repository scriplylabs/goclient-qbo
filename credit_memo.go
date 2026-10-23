package quickbooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// CreditMemo represents a QuickBooks CreditMemo object as returned by the API.
// Read-only fields (Id, SyncToken, MetaData, TotalAmt, RemainingCredit, Balance)
// are populated by the service.
type CreditMemo struct {
	ID           string        `json:"Id,omitempty"`
	SyncToken    string        `json:",omitempty"`
	MetaData     *MetaData     `json:",omitempty"`
	DocNumber    *string       `json:",omitempty"`
	TxnDate      *Date         `json:",omitempty"`
	CustomerRef  ReferenceType `json:",omitempty"`
	CustomerMemo *MemoRef      `json:",omitempty"`
	PrivateNote  *string       `json:",omitempty"`
	ProjectRef   *ReferenceType `json:",omitempty"`
	BillAddr     *Address      `json:",omitempty"`
	ShipAddr     *Address      `json:",omitempty"`
	EmailStatus  *string       `json:",omitempty"`
	BillEmail    *EmailAddress `json:",omitempty"`
	Line         []Line        `json:",omitempty"`
	TxnTaxDetail *TxnTaxDetail `json:",omitempty"`
	ApplyTaxAfterDiscount *bool `json:",omitempty"`
	CustomField  []CustomField `json:",omitempty"`
	TotalAmt     json.Number   `json:",omitempty"`
	RemainingCredit json.Number `json:",omitempty"`
	Balance      json.Number   `json:",omitempty"`
}

// CreditMemoCreateInput contains the writable fields accepted when creating a CreditMemo.
// CustomerRef and Line are required; all other fields are optional.
type CreditMemoCreateInput struct {
	CustomerRef  ReferenceType  `json:",omitempty"`
	Line         []Line
	DocNumber    *string        `json:",omitempty"`
	TxnDate      *Date          `json:",omitempty"`
	CustomerMemo *MemoRef       `json:",omitempty"`
	PrivateNote  *string        `json:",omitempty"`
	ProjectRef   *ReferenceType `json:",omitempty"`
	BillAddr     *Address       `json:",omitempty"`
	ShipAddr     *Address       `json:",omitempty"`
	EmailStatus  *string        `json:",omitempty"`
	BillEmail    *EmailAddress  `json:",omitempty"`
	TxnTaxDetail *TxnTaxDetail  `json:",omitempty"`
	ApplyTaxAfterDiscount *bool `json:",omitempty"`
	CustomField  []CustomField  `json:",omitempty"`
}

// CreateCreditMemo creates the given CreditMemo within QuickBooks.
func (c *Client) CreateCreditMemo(input *CreditMemoCreateInput) (*CreditMemo, error) {
	var resp struct {
		CreditMemo CreditMemo
		Time       Date
	}

	if err := c.post("creditmemo", input, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.CreditMemo, nil
}

// DeleteCreditMemo deletes the given credit memo.
func (c *Client) DeleteCreditMemo(creditMemo *CreditMemo) error {
	if creditMemo.ID == "" || creditMemo.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post("creditmemo", creditMemo, nil, map[string]string{"operation": "delete"})
}

// FindCreditMemos retrieves the full list of credit memos from QuickBooks.
func (c *Client) FindCreditMemos() ([]CreditMemo, error) {
	var resp struct {
		QueryResponse struct {
			CreditMemos   []CreditMemo `json:"CreditMemo"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM CreditMemo", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no credit memos could be found")
	}

	creditMemos := make([]CreditMemo, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM CreditMemo ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.CreditMemos == nil {
			return nil, errors.New("no credit memos could be found")
		}

		creditMemos = append(creditMemos, resp.QueryResponse.CreditMemos...)
	}

	return creditMemos, nil
}

// FindCreditMemoByID retrieves the given credit memo from QuickBooks.
func (c *Client) FindCreditMemoByID(id string) (*CreditMemo, error) {
	var resp struct {
		CreditMemo CreditMemo
		Time       Date
	}

	if err := c.get("creditmemo/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.CreditMemo, nil
}

// QueryCreditMemos accepts an SQL query and returns all credit memos found using it.
func (c *Client) QueryCreditMemos(query string) ([]CreditMemo, error) {
	var resp struct {
		QueryResponse struct {
			CreditMemos   []CreditMemo `json:"CreditMemo"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.CreditMemos == nil {
		return nil, errors.New("could not find any credit memos")
	}

	return resp.QueryResponse.CreditMemos, nil
}

// ListCreditMemos returns one page of CreditMemos ordered by Id.
// Pass an empty pageToken to start from the beginning.
// The returned nextPageToken is empty when there are no more results.
func (c *Client) ListCreditMemos(pageToken string, pageSize int) (*ListResponse[CreditMemo], error) {
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
			CreditMemos   []CreditMemo `json:"CreditMemo"`
			StartPosition int
			MaxResults    int
		}
	}

	query := "SELECT * FROM CreditMemo ORDERBY Id STARTPOSITION " + strconv.Itoa(startPosition) + " MAXRESULTS " + strconv.Itoa(pageSize)
	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	result := &ListResponse[CreditMemo]{Items: resp.QueryResponse.CreditMemos}
	if len(result.Items) == pageSize {
		result.NextPageToken = strconv.Itoa(startPosition + pageSize)
	}

	return result, nil
}

// UpdateCreditMemo updates the given credit memo.
func (c *Client) UpdateCreditMemo(creditMemo *CreditMemo) (*CreditMemo, error) {
	if creditMemo.ID == "" {
		return nil, errors.New("missing credit memo id")
	}

	existingCreditMemo, err := c.FindCreditMemoByID(creditMemo.ID)
	if err != nil {
		return nil, err
	}

	creditMemo.SyncToken = existingCreditMemo.SyncToken

	payload := struct {
		*CreditMemo
		Sparse bool `json:"sparse"`
	}{
		CreditMemo: creditMemo,
		Sparse:     true,
	}

	var creditMemoData struct {
		CreditMemo CreditMemo
		Time       Date
	}

	if err = c.post("creditmemo", payload, &creditMemoData, nil); err != nil {
		return nil, err
	}

	return &creditMemoData.CreditMemo, err
}
