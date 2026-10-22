package quickbooks

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetChangedEntities(t *testing.T) {
	const response = `{
 "CDCResponse": [
  {
   "QueryResponse": [
    {
     "Estimate": [
      {"domain":"QBO","sparse":false,"Id":"48","SyncToken":"0","MetaData":{"CreateTime":"2026-01-31T11:42:38-08:00","LastUpdatedTime":"2026-01-31T11:43:20-08:00"},"TxnDate":"2026-01-29","TxnStatus":"Closed","TotalAmt":1005,"CustomerRef":{"value":"18","name":"Paulsen Medical Supplies"}},
      {"domain":"QBO","sparse":false,"Id":"46","SyncToken":"0","MetaData":{"CreateTime":"2026-01-31T11:36:01-08:00","LastUpdatedTime":"2026-02-01T12:42:59-08:00"},"TxnDate":"2026-01-29","TxnStatus":"Closed","TotalAmt":70,"CustomerRef":{"value":"20","name":"Red Rock Diner"}},
      {"Id":"99","status":"deleted","MetaData":{"LastUpdatedTime":"2026-02-01T10:00:00-08:00"}}
     ],
     "startPosition": 1,
     "maxResults": 3
    },
    {
     "Customer": [
      {"Id":"13","SyncToken":"0","MetaData":{"CreateTime":"2026-01-25T17:06:42-08:00","LastUpdatedTime":"2026-01-31T11:06:49-08:00"},"GivenName":"John","FamilyName":"Melton","DisplayName":"John Melton","Active":true},
      {"Id":"29","SyncToken":"0","MetaData":{"CreateTime":"2026-01-25T17:29:04-08:00","LastUpdatedTime":"2026-01-31T11:09:08-08:00"},"GivenName":"Nicola","FamilyName":"Weiskopf","DisplayName":"Weiskopf Consulting","Active":true},
      {"Id":"5","status":"deleted","MetaData":{"LastUpdatedTime":"2026-02-01T09:00:00-08:00"}}
     ],
     "startPosition": 1,
     "maxResults": 3
    }
   ]
  }
 ],
 "time": "2026-02-28T18:20:10.657-08:00"
}`

	client, _ := NewTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/company/test-realm/cdc", r.URL.Path)
		assert.Equal(t, "Estimate,Customer", r.URL.Query().Get("entities"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	})

	result, err := client.GetChangedEntities(
		[]string{"Estimate", "Customer"},
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	// estimates: 2 live, 1 deleted
	require.Len(t, result.Estimates, 3)
	assert.Nil(t, result.Estimates[0].Deleted)
	assert.NotNil(t, result.Estimates[0].Entity)
	assert.Equal(t, "48", result.Estimates[0].Entity.ID)
	assert.Equal(t, json.Number("1005"), result.Estimates[0].Entity.TotalAmt)
	assert.Nil(t, result.Estimates[1].Deleted)
	assert.NotNil(t, result.Estimates[1].Entity)
	assert.Equal(t, "46", result.Estimates[1].Entity.ID)
	assert.NotNil(t, result.Estimates[2].Deleted)
	assert.Nil(t, result.Estimates[2].Entity)
	assert.Equal(t, "99", result.Estimates[2].Deleted.ID)
	assert.Equal(t, "deleted", result.Estimates[2].Deleted.Status)

	// customers: 2 live, 1 deleted
	require.Len(t, result.Customers, 3)
	assert.Nil(t, result.Customers[0].Deleted)
	assert.NotNil(t, result.Customers[0].Entity)
	assert.Equal(t, "13", result.Customers[0].Entity.ID)
	assert.Equal(t, "John Melton", result.Customers[0].Entity.DisplayName)
	assert.NotNil(t, result.Customers[2].Deleted)
	assert.Nil(t, result.Customers[2].Entity)
	assert.Equal(t, "5", result.Customers[2].Deleted.ID)

	// other entity types should be empty
	assert.Empty(t, result.Invoices)
	assert.Empty(t, result.Bills)
	assert.Empty(t, result.Vendors)
}

// QuickBooks capitalises the status of a deleted entity (sandbox, 2026-10-22).
func TestGetChangedEntitiesCapitalisedDeleted(t *testing.T) {
	const response = `{"CDCResponse":[{"QueryResponse":[{"Payment":[
		{"domain":"QBO","status":"Deleted","Id":"154","MetaData":{"LastUpdatedTime":"2026-10-22T11:41:17-07:00"}}
	],"maxResults":1,"totalCount":1}]}],"time":"2026-10-22T11:50:00.000-07:00"}`

	client, _ := NewTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	})

	result, err := client.GetChangedEntities([]string{"Payment"}, time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, result.Payments, 1)
	assert.Nil(t, result.Payments[0].Entity)
	require.NotNil(t, result.Payments[0].Deleted)
	assert.Equal(t, "154", result.Payments[0].Deleted.ID)
}
