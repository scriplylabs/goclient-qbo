package quickbooks

import (
	"fmt"
	"strconv"
	"time"
)

type CustomField struct {
	DefinitionID string `json:"DefinitionId,omitempty"`
	StringValue  string `json:"StringValue,omitempty"`
	Type         string `json:"Type,omitempty"`
	Name         string `json:"Name,omitempty"`
}

// Date represents a Quickbooks date (xs:date), such as TxnDate.
// It is written as "2006-01-02"; QuickBooks ignores any time of day sent with one.
type Date struct {
	time.Time `json:",omitempty"`
}

// MarshalJSON writes the date without a time of day.
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Format(secondFormat) + `"`), nil
}

// UnmarshalJSON accepts "2006-01-02" or an RFC 3339 date and time.
func (d *Date) UnmarshalJSON(b []byte) (err error) {
	if b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	}

	d.Time, err = time.Parse(time.RFC3339, string(b))
	if err != nil {
		d.Time, err = time.Parse(secondFormat, string(b))
	}

	return err
}

func (d Date) String() string {
	return d.Format(format)
}

// DateTime represents a Quickbooks date and time (xs:dateTime), such as
// MetaData.CreateTime or TimeActivity.StartTime. It is written as RFC 3339.
type DateTime struct {
	time.Time `json:",omitempty"`
}

// MarshalJSON writes the date and time as RFC 3339.
func (d DateTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Format(time.RFC3339Nano) + `"`), nil
}

// UnmarshalJSON accepts an RFC 3339 string, a "2006-01-02" string, or a number
// of milliseconds since the Unix epoch, which QuickBooks sends as the time of
// an authentication fault.
func (d *DateTime) UnmarshalJSON(b []byte) (err error) {
	if string(b) == "null" {
		return nil
	}

	if b[0] != '"' {
		ms, err := strconv.ParseInt(string(b), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid date and time %s: %v", b, err)
		}

		d.Time = time.UnixMilli(ms)

		return nil
	}

	b = b[1 : len(b)-1]

	d.Time, err = time.Parse(time.RFC3339, string(b))
	if err != nil {
		d.Time, err = time.Parse(secondFormat, string(b))
	}

	return err
}

func (d DateTime) String() string {
	return d.Format(format)
}

// EmailAddress represents a QuickBooks email address.
type EmailAddress struct {
	Address *string `json:",omitempty"`
}

// EndpointURL specifies the endpoint to connect to
type EndpointURL string

const (
	// DiscoveryProductionEndpoint is for live apps.
	DiscoveryProductionEndpoint EndpointURL = "https://developer.api.intuit.com/.well-known/openid_configuration"
	// DiscoverySandboxEndpoint is for testing.
	DiscoverySandboxEndpoint EndpointURL = "https://developer.api.intuit.com/.well-known/openid_sandbox_configuration"
	// ProductionEndpoint is for live apps.
	ProductionEndpoint EndpointURL = "https://quickbooks.api.intuit.com"
	// SandboxEndpoint is for testing.
	SandboxEndpoint EndpointURL = "https://sandbox-quickbooks.api.intuit.com"

	format        = "2006-01-02T15:04:05-07:00"
	queryPageSize = 1000
	secondFormat  = "2006-01-02"
)

// ListResponse is returned by paginated List methods.
// NextPageToken is empty when there are no more results.
type ListResponse[T any] struct {
	Items         []T
	NextPageToken string
}

func (u EndpointURL) String() string {
	return string(u)
}

// MemoRef represents a QuickBooks MemoRef object.
type MemoRef struct {
	Value string `json:"value,omitempty"`
}

// MetaData is a timestamp of genesis and last change of a Quickbooks object
type MetaData struct {
	CreateTime      DateTime `json:",omitempty"`
	LastUpdatedTime DateTime `json:",omitempty"`
}

// Address represents a QuickBooks address.
type Address struct {
	ID string `json:"Id,omitempty"`
	// These lines are context-dependent! Read the QuickBooks API carefully.
	Line1   string  `json:",omitempty"`
	Line2   *string `json:",omitempty"`
	Line3   *string `json:",omitempty"`
	Line4   *string `json:",omitempty"`
	Line5   *string `json:",omitempty"`
	City    string  `json:",omitempty"`
	Country string  `json:",omitempty"`
	// A.K.A. State.
	CountrySubDivisionCode string `json:",omitempty"`
	PostalCode             string `json:",omitempty"`
	Lat                    string `json:",omitempty"`
	Long                   string `json:",omitempty"`
}

type NameValue struct {
	Value string `json:"value,omitempty"`
	Name  string `json:"name,omitempty"`
}

// ReferenceType represents a QuickBooks reference to another object.
type ReferenceType struct {
	NameValue
	Type string `json:"type,omitempty"`
}

// TelephoneNumber represents a QuickBooks phone number.
type TelephoneNumber struct {
	FreeFormNumber string `json:",omitempty"`
}

// WebSiteAddress represents a Quickbooks Website
type WebSiteAddress struct {
	URI string `json:",omitempty"`
}
