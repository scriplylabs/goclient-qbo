package quickbooks

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bodies below are QuickBooks sandbox responses captured on 2026-10-22.
func TestParseFailure(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantType string
		wantCode string
		wantTime int64 // Unix milliseconds
	}{
		{
			name:     "authentication fault, lowercase keys and epoch time",
			status:   http.StatusUnauthorized,
			body:     `{"warnings":null,"intuitObject":null,"fault":{"error":[{"message":"message=AuthenticationFailed; errorCode=003200; statusCode=401","detail":"Malformed bearer token: too short or too long","code":"3200","element":null}],"type":"AUTHENTICATION"},"report":null,"queryResponse":null,"batchItemResponse":[],"attachableResponse":[],"syncErrorResponse":null,"requestId":null,"time":1792694440314,"status":null,"cdcresponse":[]}`,
			wantType: "AUTHENTICATION",
			wantCode: "3200",
			wantTime: 1792694440314,
		},
		{
			name:     "scope fault",
			status:   http.StatusForbidden,
			body:     `{"warnings":null,"intuitObject":null,"fault":{"error":[{"message":"message=AuthorizationFailed; errorCode=003100; statusCode=403","detail":"The access token does not have sufficient scope","code":"3100","element":null}],"type":"SERVICE"},"report":null,"queryResponse":null,"batchItemResponse":[],"attachableResponse":[],"syncErrorResponse":null,"requestId":null,"time":1792693957260,"status":null,"cdcresponse":[]}`,
			wantType: "SERVICE",
			wantCode: "3100",
			wantTime: 1792693957260,
		},
		{
			name:     "validation fault, account period closed",
			status:   http.StatusBadRequest,
			body:     `{"Fault":{"Error":[{"Message":"Account Period Closed","Detail":"The account period has closed. Txn Date=06/15/2026 is before book closing date=06/30/2026","code":"6200","element":""}],"type":"ValidationFault"},"time":"2026-10-22T11:46:08.476-07:00"}`,
			wantType: "ValidationFault",
			wantCode: "6200",
			wantTime: 1792694768476,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := NewTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			_, err := client.FindPreferences()

			var failure Failure
			require.True(t, errors.As(err, &failure), "want a Failure, got %T: %v", err, err)
			assert.Equal(t, tt.wantType, failure.Fault.Type)
			require.Len(t, failure.Fault.Error, 1)
			assert.Equal(t, tt.wantCode, failure.Fault.Error[0].Code)
			assert.NotEmpty(t, failure.Fault.Error[0].Message)
			assert.Equal(t, tt.wantTime, failure.Time.UnixMilli())
		})
	}
}
