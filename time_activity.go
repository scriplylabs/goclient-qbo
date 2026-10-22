package quickbooks

import (
	"encoding/json"
	"errors"
	"strconv"
)

// TimeActivity represents a QuickBooks TimeActivity object as returned by the API.
// Read-only fields (Id, SyncToken, MetaData) are populated by the service.
type TimeActivity struct {
	ID             string         `json:"Id,omitempty"`
	SyncToken      string         `json:",omitempty"`
	MetaData       *MetaData      `json:",omitempty"`
	NameOf         string         `json:",omitempty"`
	EmployeeRef    *ReferenceType `json:",omitempty"`
	VendorRef      *ReferenceType `json:",omitempty"`
	CustomerRef    *ReferenceType `json:",omitempty"`
	ItemRef        *ReferenceType `json:",omitempty"`
	ClassRef       *ReferenceType `json:",omitempty"`
	DepartmentRef  *ReferenceType `json:",omitempty"`
	TaxCodeRef     *ReferenceType `json:",omitempty"`
	BillableStatus *string        `json:",omitempty"`
	Taxable        *bool          `json:",omitempty"`
	HourlyRate     json.Number    `json:",omitempty"`
	TxnDate        *Date          `json:",omitempty"`
	Hours          *int           `json:",omitempty"`
	Minutes        *int           `json:",omitempty"`
	StartTime      *DateTime      `json:",omitempty"`
	EndTime        *DateTime      `json:",omitempty"`
	BreakHours     *int           `json:",omitempty"`
	BreakMinutes   *int           `json:",omitempty"`
	Description    *string        `json:",omitempty"`
}

// TimeActivityCreateInput contains the writable fields accepted when creating a TimeActivity.
// NameOf and the corresponding entity ref (EmployeeRef or VendorRef) are required.
type TimeActivityCreateInput struct {
	NameOf         string         `json:",omitempty"`
	EmployeeRef    *ReferenceType `json:",omitempty"`
	VendorRef      *ReferenceType `json:",omitempty"`
	CustomerRef    *ReferenceType `json:",omitempty"`
	ItemRef        *ReferenceType `json:",omitempty"`
	ClassRef       *ReferenceType `json:",omitempty"`
	DepartmentRef  *ReferenceType `json:",omitempty"`
	TaxCodeRef     *ReferenceType `json:",omitempty"`
	BillableStatus *string        `json:",omitempty"`
	Taxable        *bool          `json:",omitempty"`
	HourlyRate     json.Number    `json:",omitempty"`
	TxnDate        *Date          `json:",omitempty"`
	Hours          *int           `json:",omitempty"`
	Minutes        *int           `json:",omitempty"`
	StartTime      *DateTime      `json:",omitempty"`
	EndTime        *DateTime      `json:",omitempty"`
	BreakHours     *int           `json:",omitempty"`
	BreakMinutes   *int           `json:",omitempty"`
	Description    *string        `json:",omitempty"`
}

// CreateTimeActivity creates the given TimeActivity on the QuickBooks server, returning
// the resulting TimeActivity object.
func (c *Client) CreateTimeActivity(input *TimeActivityCreateInput) (*TimeActivity, error) {
	var resp struct {
		TimeActivity TimeActivity
		Time         Date
	}

	if err := c.post("timeactivity", input, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.TimeActivity, nil
}

// DeleteTimeActivity deletes the time activity.
func (c *Client) DeleteTimeActivity(timeActivity *TimeActivity) error {
	if timeActivity.ID == "" || timeActivity.SyncToken == "" {
		return errors.New("missing id/sync token")
	}

	return c.post("timeactivity", timeActivity, nil, map[string]string{"operation": "delete"})
}

// FindTimeActivities gets the full list of TimeActivities in the QuickBooks account.
func (c *Client) FindTimeActivities() ([]TimeActivity, error) {
	var resp struct {
		QueryResponse struct {
			TimeActivities []TimeActivity `json:"TimeActivity"`
			MaxResults     int
			StartPosition  int
			TotalCount     int
		}
	}

	if err := c.query("SELECT COUNT(*) FROM TimeActivity", &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TotalCount == 0 {
		return nil, errors.New("no time activities could be found")
	}

	timeActivities := make([]TimeActivity, 0, resp.QueryResponse.TotalCount)

	for i := 0; i < resp.QueryResponse.TotalCount; i += queryPageSize {
		query := "SELECT * FROM TimeActivity ORDERBY Id STARTPOSITION " + strconv.Itoa(i+1) + " MAXRESULTS " + strconv.Itoa(queryPageSize)

		if err := c.query(query, &resp); err != nil {
			return nil, err
		}

		if resp.QueryResponse.TimeActivities == nil {
			return nil, errors.New("no time activities could be found")
		}

		timeActivities = append(timeActivities, resp.QueryResponse.TimeActivities...)
	}

	return timeActivities, nil
}

// FindTimeActivityByID returns a time activity with a given Id.
func (c *Client) FindTimeActivityByID(id string) (*TimeActivity, error) {
	var resp struct {
		TimeActivity TimeActivity
		Time         Date
	}

	if err := c.get("timeactivity/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.TimeActivity, nil
}

// QueryTimeActivities accepts an SQL query and returns all time activities found using it.
func (c *Client) QueryTimeActivities(query string) ([]TimeActivity, error) {
	var resp struct {
		QueryResponse struct {
			TimeActivities []TimeActivity `json:"TimeActivity"`
			StartPosition  int
			MaxResults     int
		}
	}

	if err := c.query(query, &resp); err != nil {
		return nil, err
	}

	if resp.QueryResponse.TimeActivities == nil {
		return nil, errors.New("could not find any time activities")
	}

	return resp.QueryResponse.TimeActivities, nil
}

// UpdateTimeActivity updates the time activity.
func (c *Client) UpdateTimeActivity(timeActivity *TimeActivity) (*TimeActivity, error) {
	if timeActivity.ID == "" {
		return nil, errors.New("missing time activity id")
	}

	existingTimeActivity, err := c.FindTimeActivityByID(timeActivity.ID)
	if err != nil {
		return nil, err
	}

	timeActivity.SyncToken = existingTimeActivity.SyncToken

	payload := struct {
		*TimeActivity
		Sparse bool `json:"sparse"`
	}{
		TimeActivity: timeActivity,
		Sparse:       true,
	}

	var timeActivityData struct {
		TimeActivity TimeActivity
		Time         Date
	}

	if err = c.post("timeactivity", payload, &timeActivityData, nil); err != nil {
		return nil, err
	}

	return &timeActivityData.TimeActivity, err
}
