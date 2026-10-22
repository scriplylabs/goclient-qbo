package quickbooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// JournalEntry represents a QuickBooks JournalEntry object as returned by the API.
// Read-only fields (Id, SyncToken, MetaData, TotalAmt, HomeTotalAmt) are populated by the service.
type JournalEntry struct {
	ID           string        `json:"Id,omitempty"`
	SyncToken    string        `json:",omitempty"`
	MetaData     *MetaData     `json:",omitempty"`
	DocNumber    *string       `json:",omitempty"`
	TxnDate      *Date         `json:",omitempty"`
	PrivateNote  *string       `json:",omitempty"`
	Line         []Line        `json:",omitempty"`
	CurrencyRef  *ReferenceType `json:",omitempty"`
	ExchangeRate json.Number   `json:",omitempty"`
	TxnTaxDetail *TxnTaxDetail `json:",omitempty"`
	Adjustment   *bool         `json:",omitempty"`
	TotalAmt     json.Number   `json:",omitempty"`
	HomeTotalAmt json.Number   `json:",omitempty"`
}

// JournalEntryCreateInput contains the writable fields accepted when creating a JournalEntry.
// Line is required (must contain balanced debit and credit entries).
type JournalEntryCreateInput struct {
	Line         []Line         `json:",omitempty"`
	DocNumber    *string        `json:",omitempty"`
	TxnDate      *Date          `json:",omitempty"`
	PrivateNote  *string        `json:",omitempty"`
	CurrencyRef  *ReferenceType `json:",omitempty"`
	ExchangeRate json.Number    `json:",omitempty"`
	TxnTaxDetail *TxnTaxDetail  `json:",omitempty"`
	Adjustment   *bool          `json:",omitempty"`
}

// CreateJournalEntry creates the given JournalEntry on the QuickBooks server, returning
// the resulting JournalEntry object.
//
// Pass WithRequestID to make retries of the same create safe.
func (c *Client) CreateJournalEntry(input *JournalEntryCreateInput, opts ...RequestOption) (*JournalEntry, error) {
	var resp struct {
		JournalEntry JournalEntry
		Time         Date
	}

	if err := c.post("journalentry", input, &resp, buildQueryParameters(opts)); err != nil {
		return nil, err
	}

	return &resp.JournalEntry, nil
}

// DeleteJournalEntry deletes the journal entry.
func (c *Client) DeleteJournalEntry(journalEntry *JournalEntry) error {
	if journalEntry.ID == "" || journalEntry.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post("journalentry", journalEntry, nil, map[string]string{"operation": "delete"})
}

// FindJournalEntries gets the full list of JournalEntries in the QuickBooks account.
func (c *Client) FindJournalEntries() ([]JournalEntry, error) {
	var resp struct {
		QueryResponse struct {
			JournalEntries []JournalEntry `json:"JournalEntry"`
			MaxResults     int
			StartPosition  int
			TotalCount     int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM JournalEntry", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no journal entries could be found")
	}

	journalEntries := make([]JournalEntry, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM JournalEntry ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.JournalEntries == nil {
			return nil, errors.New("no journal entries could be found")
		}

		journalEntries = append(journalEntries, resp.QueryResponse.JournalEntries...)
	}

	return journalEntries, nil
}

// FindJournalEntryByID finds the journal entry by the given id.
func (c *Client) FindJournalEntryByID(id string) (*JournalEntry, error) {
	var resp struct {
		JournalEntry JournalEntry
		Time         Date
	}

	if err := c.get("journalentry/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.JournalEntry, nil
}

// QueryJournalEntries accepts an SQL query and returns all journal entries found using it.
func (c *Client) QueryJournalEntries(query string) ([]JournalEntry, error) {
	var resp struct {
		QueryResponse struct {
			JournalEntries []JournalEntry `json:"JournalEntry"`
			StartPosition  int
			MaxResults     int
		}
	}

	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.JournalEntries == nil {
		return nil, errors.New("could not find any journal entries")
	}

	return resp.QueryResponse.JournalEntries, nil
}

// ListJournalEntries returns one page of JournalEntries ordered by Id.
// Pass an empty pageToken to start from the beginning.
// The returned nextPageToken is empty when there are no more results.
func (c *Client) ListJournalEntries(pageToken string, pageSize int) (*ListResponse[JournalEntry], error) {
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
			JournalEntries []JournalEntry `json:"JournalEntry"`
			StartPosition  int
			MaxResults     int
		}
	}

	query := "SELECT * FROM JournalEntry ORDERBY Id STARTPOSITION " + strconv.Itoa(startPosition) + " MAXRESULTS " + strconv.Itoa(pageSize)
	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	result := &ListResponse[JournalEntry]{Items: resp.QueryResponse.JournalEntries}
	if len(result.Items) == pageSize {
		result.NextPageToken = strconv.Itoa(startPosition + pageSize)
	}

	return result, nil
}

// UpdateJournalEntry updates the journal entry.
func (c *Client) UpdateJournalEntry(journalEntry *JournalEntry) (*JournalEntry, error) {
	if journalEntry.ID == "" {
		return nil, errors.New("missing journal entry id")
	}

	existingJournalEntry, err := c.FindJournalEntryByID(journalEntry.ID)
	if err != nil {
		return nil, err
	}

	journalEntry.SyncToken = existingJournalEntry.SyncToken

	payload := struct {
		*JournalEntry
		Sparse bool `json:"sparse"`
	}{
		JournalEntry: journalEntry,
		Sparse:       true,
	}

	var journalEntryData struct {
		JournalEntry JournalEntry
		Time         Date
	}

	if err = c.post("journalentry", payload, &journalEntryData, nil); err != nil {
		return nil, err
	}

	return &journalEntryData.JournalEntry, err
}
