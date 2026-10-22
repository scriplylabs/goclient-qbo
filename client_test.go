package quickbooks

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRequestID(t *testing.T) {
	var gotRequestIDs []string
	client, _ := NewTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotRequestIDs = append(gotRequestIDs, r.URL.Query().Get("requestid"))
		assert.Equal(t, "65", r.URL.Query().Get("minorversion"))
		_, _ = w.Write([]byte(`{"JournalEntry":{"Id":"7"},"Payment":{"Id":"8"},"BillPayment":{"Id":"9"}}`))
	})

	_, err := client.CreateJournalEntry(&JournalEntryCreateInput{}, WithRequestID("je-1"))
	require.NoError(t, err)
	_, err = client.CreatePayment(&PaymentCreateInput{}, WithRequestID("pay-1"))
	require.NoError(t, err)
	_, err = client.CreateBillPayment(&BillPaymentCreateInput{}, WithRequestID("bp-1"))
	require.NoError(t, err)
	_, err = client.CreateJournalEntry(&JournalEntryCreateInput{})
	require.NoError(t, err)

	assert.Equal(t, []string{"je-1", "pay-1", "bp-1", ""}, gotRequestIDs)
}

func TestNewClientWithEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/company/123/journalentry", r.URL.Path)
		assert.Equal(t, "70", r.URL.Query().Get("minorversion"))
		assert.Equal(t, "Bearer access", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		assert.JSONEq(t, `{"PrivateNote":"x"}`, string(body))
		_ = json.NewEncoder(w).Encode(map[string]any{"JournalEntry": map[string]string{"Id": "7"}})
	}))
	t.Cleanup(server.Close)

	client, err := NewClientWithEndpoint(EndpointURL(server.URL+"/"), "123", "70", &BearerToken{AccessToken: "access"})
	require.NoError(t, err)

	note := "x"
	je, err := client.CreateJournalEntry(&JournalEntryCreateInput{PrivateNote: &note})
	require.NoError(t, err)
	assert.Equal(t, "7", je.ID)

	_, err = client.FindAuthorizationURL("com.intuit.quickbooks.accounting", "state", "https://example.com")
	assert.ErrorContains(t, err, "no OAuth endpoints")
	_, err = client.RefreshToken("refresh")
	assert.ErrorContains(t, err, "no OAuth endpoints")
	assert.ErrorContains(t, client.RevokeToken("refresh"), "no OAuth endpoints")
}
