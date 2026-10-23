package quickbooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// Deposit represents a QuickBooks Deposit object as returned by the API.
// Read-only fields (Id, SyncToken, MetaData, TotalAmt) are populated by the service.
type Deposit struct {
	ID                  string        `json:"Id,omitempty"`
	SyncToken           string        `json:",omitempty"`
	MetaData            *MetaData     `json:",omitempty"`
	DepositToAccountRef ReferenceType `json:",omitempty"`
	TxnDate             *Date         `json:",omitempty"`
	PrivateNote         *string       `json:",omitempty"`
	TotalAmt            json.Number   `json:",omitempty"`
	Line                []PaymentLine `json:",omitempty"`
}

// DepositCreateInput contains the writable fields accepted when creating a Deposit.
// DepositToAccountRef and Line are required; all other fields are optional.
type DepositCreateInput struct {
	DepositToAccountRef ReferenceType `json:",omitempty"`
	Line                []PaymentLine
	TxnDate             *Date   `json:",omitempty"`
	PrivateNote         *string `json:",omitempty"`
}

// CreateDeposit creates the given deposit within QuickBooks
func (c *Client) CreateDeposit(input *DepositCreateInput) (*Deposit, error) {
	var resp struct {
		Deposit Deposit
		Time    Date
	}

	if err := c.post("deposit", input, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Deposit, nil
}

func (c *Client) DeleteDeposit(deposit *Deposit) error {
	if deposit.ID == "" || deposit.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post("deposit", deposit, nil, map[string]string{"operation": "delete"})
}

// FindDeposits gets the full list of Deposits in the QuickBooks account.
func (c *Client) FindDeposits() ([]Deposit, error) {
	var resp struct {
		QueryResponse struct {
			Deposits      []Deposit `json:"Deposit"`
			MaxResults    int
			StartPosition int
			TotalCount    int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM Deposit", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no deposits could be found")
	}

	deposits := make([]Deposit, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM Deposit ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.Deposits == nil {
			return nil, errors.New("no deposits could be found")
		}

		deposits = append(deposits, resp.QueryResponse.Deposits...)
	}

	return deposits, nil
}

// FindDepositByID returns a deposit with a given Id.
func (c *Client) FindDepositByID(id string) (*Deposit, error) {
	var resp struct {
		Deposit Deposit
		Time    Date
	}

	if err := c.get("deposit/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Deposit, nil
}

// QueryDeposits accepts an SQL query and returns all deposits found using it
func (c *Client) QueryDeposits(query string) ([]Deposit, error) {
	var resp struct {
		QueryResponse struct {
			Deposits      []Deposit `json:"Deposit"`
			StartPosition int
			MaxResults    int
		}
	}

	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.Deposits == nil {
		return nil, errors.New("could not find any deposits")
	}

	return resp.QueryResponse.Deposits, nil
}

// ListDeposits returns one page of Deposits ordered by Id.
// Pass an empty pageToken to start from the beginning.
// The returned nextPageToken is empty when there are no more results.
func (c *Client) ListDeposits(pageToken string, pageSize int) (*ListResponse[Deposit], error) {
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
			Deposits      []Deposit `json:"Deposit"`
			StartPosition int
			MaxResults    int
		}
	}

	query := "SELECT * FROM Deposit ORDERBY Id STARTPOSITION " + strconv.Itoa(startPosition) + " MAXRESULTS " + strconv.Itoa(pageSize)
	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	result := &ListResponse[Deposit]{Items: resp.QueryResponse.Deposits}
	if len(result.Items) == pageSize {
		result.NextPageToken = strconv.Itoa(startPosition + pageSize)
	}

	return result, nil
}

// UpdateDeposit updates the deposit
func (c *Client) UpdateDeposit(deposit *Deposit) (*Deposit, error) {
	if deposit.ID == "" {
		return nil, errors.New("missing deposit id")
	}

	existingDeposit, err := c.FindDepositByID(deposit.ID)
	if err != nil {
		return nil, err
	}

	deposit.SyncToken = existingDeposit.SyncToken

	payload := struct {
		*Deposit
		Sparse bool `json:"sparse"`
	}{
		Deposit: deposit,
		Sparse:  true,
	}

	var depositData struct {
		Deposit Deposit
		Time    Date
	}

	if err = c.post("deposit", payload, &depositData, nil); err != nil {
		return nil, err
	}

	return &depositData.Deposit, err
}
