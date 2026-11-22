package quickbooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CDCResponse holds the results of a Change Data Capture query,
// with changed entities aggregated by type.
type CDCResponse struct {
	Accounts      []MaybeDeleted[Account]
	Attachables   []MaybeDeleted[Attachable]
	Bills         []MaybeDeleted[Bill]
	BillPayments  []MaybeDeleted[BillPayment]
	Classes       []MaybeDeleted[Class]
	CompanyInfos  []MaybeDeleted[CompanyInfo]
	CreditMemos   []MaybeDeleted[CreditMemo]
	Customers     []MaybeDeleted[Customer]
	CustomerTypes []MaybeDeleted[CustomerType]
	Deposits      []MaybeDeleted[Deposit]
	Employees     []MaybeDeleted[Employee]
	Estimates     []MaybeDeleted[Estimate]
	Invoices      []MaybeDeleted[Invoice]
	Items         []MaybeDeleted[Item]
	Payments      []MaybeDeleted[Payment]
	Vendors       []MaybeDeleted[Vendor]
}

func unmarshalEntities[T any](dest *[]MaybeDeleted[T], data json.RawMessage) error {
	var entities []MaybeDeleted[T]
	if err := json.Unmarshal(data, &entities); err != nil {
		return err
	}
	*dest = append(*dest, entities...)
	return nil
}

func buildDispatch(result *CDCResponse) map[string]func(json.RawMessage) error {
	return map[string]func(json.RawMessage) error{
		"Account":      func(d json.RawMessage) error { return unmarshalEntities(&result.Accounts, d) },
		"Attachable":   func(d json.RawMessage) error { return unmarshalEntities(&result.Attachables, d) },
		"Bill":         func(d json.RawMessage) error { return unmarshalEntities(&result.Bills, d) },
		"BillPayment":  func(d json.RawMessage) error { return unmarshalEntities(&result.BillPayments, d) },
		"Class":        func(d json.RawMessage) error { return unmarshalEntities(&result.Classes, d) },
		"CompanyInfo":  func(d json.RawMessage) error { return unmarshalEntities(&result.CompanyInfos, d) },
		"CreditMemo":   func(d json.RawMessage) error { return unmarshalEntities(&result.CreditMemos, d) },
		"Customer":     func(d json.RawMessage) error { return unmarshalEntities(&result.Customers, d) },
		"CustomerType": func(d json.RawMessage) error { return unmarshalEntities(&result.CustomerTypes, d) },
		"Deposit":      func(d json.RawMessage) error { return unmarshalEntities(&result.Deposits, d) },
		"Employee":     func(d json.RawMessage) error { return unmarshalEntities(&result.Employees, d) },
		"Estimate":     func(d json.RawMessage) error { return unmarshalEntities(&result.Estimates, d) },
		"Invoice":      func(d json.RawMessage) error { return unmarshalEntities(&result.Invoices, d) },
		"Item":         func(d json.RawMessage) error { return unmarshalEntities(&result.Items, d) },
		"Payment":      func(d json.RawMessage) error { return unmarshalEntities(&result.Payments, d) },
		"Vendor":       func(d json.RawMessage) error { return unmarshalEntities(&result.Vendors, d) },
	}
}

// knownMetaKeys are the non-entity keys that appear alongside entity data
// in a QueryResponse element.
var knownMetaKeys = map[string]bool{
	"startPosition": true,
	"maxResults":    true,
	"totalCount":    true,
}

func (c *Client) GetChangedEntities(entities []string, changedSince time.Time) (*CDCResponse, error) {
	var raw struct {
		CDCResponse []struct {
			QueryResponse []json.RawMessage `json:"QueryResponse"`
		} `json:"CDCResponse"`
		Time Date `json:"time"`
	}

	if err := c.get("cdc", &raw, map[string]string{
		"entities":     strings.Join(entities, ","),
		"changedSince": changedSince.Format(format),
	}); err != nil {
		return nil, err
	}

	result := &CDCResponse{}
	dispatch := buildDispatch(result)

	for _, cdcResp := range raw.CDCResponse {
		for _, qrRaw := range cdcResp.QueryResponse {
			var qr map[string]json.RawMessage
			if err := json.Unmarshal(qrRaw, &qr); err != nil {
				return nil, err
			}

			matched := 0
			for key, data := range qr {
				if knownMetaKeys[key] {
					continue
				}
				fn, ok := dispatch[key]
				if !ok {
					return nil, fmt.Errorf("unexpected entity type in CDC response: %q", key)
				}
				if err := fn(data); err != nil {
					return nil, fmt.Errorf("failed to unmarshal %q: %w", key, err)
				}
				matched++
			}
			if matched > 1 {
				return nil, fmt.Errorf("CDC QueryResponse element contained %d entity types, expected 1", matched)
			}
		}
	}

	return result, nil
}
