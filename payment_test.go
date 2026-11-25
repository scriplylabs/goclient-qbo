package quickbooks

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A payment with no lines is unapplied, and sends the accounts receivable
// account it is booked against.
func TestCreatePaymentSendsARAccount(t *testing.T) {
	client, _ := NewTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.JSONEq(t, `{
			"CustomerRef": {"value": "1"},
			"TotalAmt": 5.00,
			"DepositToAccountRef": {"value": "36"},
			"ARAccountRef": {"value": "84"}
		}`, string(body))
		_, _ = w.Write([]byte(`{"Payment":{"Id":"90","TotalAmt":5.00,"UnappliedAmt":5.00,"ARAccountRef":{"value":"84"}}}`))
	})

	payment, err := client.CreatePayment(&PaymentCreateInput{
		CustomerRef:         ReferenceType{NameValue: NameValue{Value: "1"}},
		TotalAmt:            "5.00",
		DepositToAccountRef: &ReferenceType{NameValue: NameValue{Value: "36"}},
		ARAccountRef:        &ReferenceType{NameValue: NameValue{Value: "84"}},
	})
	require.NoError(t, err)

	require.NotNil(t, payment.ARAccountRef)
	assert.Equal(t, "84", payment.ARAccountRef.Value)
}
