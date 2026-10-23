package quickbooks

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A deposit sends a linked line with the payment's line ID, and a direct
// line with its DepositLineDetail, each without the other's fields.
func TestCreateDepositLines(t *testing.T) {
	client, _ := NewTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.JSONEq(t, `{
			"DepositToAccountRef": {"value": "35"},
			"PrivateNote": "note",
			"Line": [
				{"Amount": 5.00, "LinkedTxn": [{"TxnId": "12", "TxnType": "Payment", "TxnLineId": "0"}]},
				{"Amount": 7.00, "DetailType": "DepositLineDetail", "DepositLineDetail": {"AccountRef": {"value": "79"}, "CheckNum": "1001"}}
			]
		}`, string(body))
		_, _ = w.Write([]byte(`{"Deposit":{"Id":"40","Line":[{"Id":"1","Amount":5.00,"LinkedTxn":[{"TxnId":"12","TxnType":"Payment","TxnLineId":"0"}]},{"Id":"2","Amount":7.00,"DetailType":"DepositLineDetail","DepositLineDetail":{"AccountRef":{"value":"79"},"CheckNum":"1001"}}]}}`))
	})
	note, zero, check := "note", "0", "1001"

	deposit, err := client.CreateDeposit(&DepositCreateInput{
		DepositToAccountRef: ReferenceType{NameValue: NameValue{Value: "35"}},
		PrivateNote:         &note,
		Line: []DepositLine{
			{Amount: "5.00", LinkedTxn: []LinkedTxn{{TxnID: "12", TxnType: "Payment", TxnLineID: &zero}}},
			{Amount: "7.00", DetailType: "DepositLineDetail", DepositLineDetail: &DepositLineDetail{AccountRef: ReferenceType{NameValue: NameValue{Value: "79"}}, CheckNum: &check}},
		},
	})
	require.NoError(t, err)

	require.Len(t, deposit.Line, 2)
	assert.Equal(t, &zero, deposit.Line[0].LinkedTxn[0].TxnLineID)
	assert.Equal(t, "79", deposit.Line[1].DepositLineDetail.AccountRef.Value)
}
