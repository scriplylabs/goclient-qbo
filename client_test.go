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

// A 429 makes nothing, so a create must fail rather than return an empty
// entity as if it had been made.
func TestRateLimitedRequestFails(t *testing.T) {
	client, _ := NewTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"Fault":{"Error":[{"Message":"message=ThrottleExceeded; errorCode=003001; statusCode=429","Detail":"Too many requests","code":"3001"}],"type":"ThrottleExceeded"}}`))
	})

	je, err := client.CreateJournalEntry(&JournalEntryCreateInput{})

	assert.Nil(t, je)
	var failure Failure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, "3001", failure.Fault.Error[0].Code)

	_, err = client.CreateJournalEntry(&JournalEntryCreateInput{})
	assert.ErrorContains(t, err, "waiting for rate limit", "throttled for a minute after")
}

type tokenSourceError struct{}

func (tokenSourceError) Error() string { return "token source failed" }

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, tokenSourceError{}
}

// A transport's error, such as an oauth2 token source's, stays matchable
// with errors.As through the client.
func TestRequestErrorKeepsItsCause(t *testing.T) {
	client, _ := NewTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	client.Client = &http.Client{Transport: failingTransport{}}

	_, err := client.CreateJournalEntry(&JournalEntryCreateInput{})

	require.ErrorAs(t, err, &tokenSourceError{})
}

// A transaction's PrivateNote is sent on create and read back from QuickBooks' answer.
func TestPrivateNoteRoundTrips(t *testing.T) {
	client, _ := NewTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "note", body["PrivateNote"])
		entity := map[string]any{"PrivateNote": body["PrivateNote"]}
		_ = json.NewEncoder(w).Encode(map[string]any{"Payment": entity, "Deposit": entity, "CreditMemo": entity, "Estimate": entity})
	})
	note := "note"

	payment, err := client.CreatePayment(&PaymentCreateInput{PrivateNote: &note})
	require.NoError(t, err)
	deposit, err := client.CreateDeposit(&DepositCreateInput{PrivateNote: &note})
	require.NoError(t, err)
	creditMemo, err := client.CreateCreditMemo(&CreditMemoCreateInput{PrivateNote: &note})
	require.NoError(t, err)
	estimate, err := client.CreateEstimate(&EstimateCreateInput{PrivateNote: &note})
	require.NoError(t, err)

	assert.Equal(t, []*string{&note, &note, &note, &note}, []*string{payment.PrivateNote, deposit.PrivateNote, creditMemo.PrivateNote, estimate.PrivateNote})
}
