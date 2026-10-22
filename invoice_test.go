package quickbooks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLineMarshalJSON(t *testing.T) {
	ref := func(v string) ReferenceType { return ReferenceType{NameValue: NameValue{Value: v}} }
	// Every detail is filled in, so a line that leaks one it shouldn't is caught.
	full := func(detailType string) Line {
		return Line{
			ID:                            "1",
			LineNum:                       2,
			Description:                   "desc",
			Amount:                        "10.00",
			DetailType:                    detailType,
			AccountBasedExpenseLineDetail: AccountBasedExpenseLineDetail{AccountRef: ref("31")},
			SalesItemLineDetail:           SalesItemLineDetail{ItemRef: &ReferenceType{NameValue: NameValue{Value: "5"}}},
			DiscountLineDetail:            DiscountLineDetail{DiscountPercent: "10"},
			TaxLineDetail:                 TaxLineDetail{TaxRateRef: ref("3")},
			JournalEntryLineDetail:        JournalEntryLineDetail{PostingType: "Debit", AccountRef: ref("79")},
			ItemBasedExpenseLineDetail:    ItemBasedExpenseLineDetail{ItemRef: ref("6")},
		}
	}
	const head = `{"Id":"1","LineNum":2,"Description":"desc","Amount":10.00,`

	tests := []struct {
		detailType string
		want       string
	}{
		{"AccountBasedExpenseLineDetail", head + `"DetailType":"AccountBasedExpenseLineDetail","AccountBasedExpenseLineDetail":{"AccountRef":{"value":"31"}}}`},
		{"SalesItemLineDetail", head + `"DetailType":"SalesItemLineDetail","SalesItemLineDetail":{"ItemRef":{"value":"5"}}}`},
		{"DiscountLineDetail", head + `"DetailType":"DiscountLineDetail","DiscountLineDetail":{"DiscountPercent":10}}`},
		{"TaxLineDetail", head + `"DetailType":"TaxLineDetail","TaxLineDetail":{"TaxRateRef":{"value":"3"}}}`},
		{"JournalEntryLineDetail", head + `"DetailType":"JournalEntryLineDetail","JournalEntryLineDetail":{"PostingType":"Debit","AccountRef":{"value":"79"}}}`},
		{"ItemBasedExpenseLineDetail", head + `"DetailType":"ItemBasedExpenseLineDetail","ItemBasedExpenseLineDetail":{"ItemRef":{"value":"6"}}}`},
		{"SubTotalLineDetail", head + `"DetailType":"SubTotalLineDetail"}`},
		{"DescriptionOnly", head + `"DetailType":"DescriptionOnly"}`},
	}

	for _, tt := range tests {
		t.Run(tt.detailType, func(t *testing.T) {
			got, err := json.Marshal(full(tt.detailType))
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(got))
		})
	}

	t.Run("GroupLineDetail with nested lines", func(t *testing.T) {
		line := Line{
			Amount:     "20.00",
			DetailType: "GroupLineDetail",
			GroupLineDetail: &GroupLineDetail{
				GroupItemRef: ref("19"),
				Line:         []Line{full("SalesItemLineDetail")},
			},
		}

		got, err := json.Marshal(line)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Amount":20.00,"DetailType":"GroupLineDetail","GroupLineDetail":{"GroupItemRef":{"value":"19"},
			"Line":[`+head+`"DetailType":"SalesItemLineDetail","SalesItemLineDetail":{"ItemRef":{"value":"5"}}}]}}`, string(got))
	})

	t.Run("journal entry create input", func(t *testing.T) {
		date := Date{}
		require.NoError(t, json.Unmarshal([]byte(`"2026-10-01"`), &date))
		got, err := json.Marshal(JournalEntryCreateInput{TxnDate: &date, Line: []Line{{
			Amount: "10.00", DetailType: "JournalEntryLineDetail",
			JournalEntryLineDetail: JournalEntryLineDetail{PostingType: "Debit", AccountRef: ref("79")},
		}}})
		require.NoError(t, err)
		assert.Equal(t, `{"Line":[{"Amount":10.00,"DetailType":"JournalEntryLineDetail","JournalEntryLineDetail":{"PostingType":"Debit","AccountRef":{"value":"79"}}}],"TxnDate":"2026-10-01"}`, string(got))
	})
}

// Decoding is untouched by Line.MarshalJSON: a QuickBooks response still fills
// the value-typed detail fields.
func TestLineUnmarshalJSON(t *testing.T) {
	var lines []Line
	require.NoError(t, json.Unmarshal([]byte(`[
		{"Id":"1","LineNum":1,"Description":"Lumber","Amount":103.55,"DetailType":"AccountBasedExpenseLineDetail",
		 "AccountBasedExpenseLineDetail":{"AccountRef":{"name":"Decks and Patios","value":"64"},"CustomerRef":{"value":"26"}}},
		{"Id":"0","Amount":4.0,"DetailType":"JournalEntryLineDetail","JournalEntryLineDetail":{"PostingType":"Debit",
		 "AccountRef":{"value":"84","name":"Accounts Receivable (A/R)"},"Entity":{"Type":"Customer","EntityRef":{"value":"1"}}},
		 "CustomExtensions":[]},
		{"Amount":20,"DetailType":"GroupLineDetail","GroupLineDetail":{"GroupItemRef":{"value":"19"},"Quantity":2,
		 "Line":[{"Amount":10,"DetailType":"SalesItemLineDetail","SalesItemLineDetail":{"ItemRef":{"value":"5"},"Qty":1}}]}}
	]`), &lines))

	require.Len(t, lines, 3)
	assert.Equal(t, Line{
		ID: "1", LineNum: 1, Description: "Lumber", Amount: "103.55", DetailType: "AccountBasedExpenseLineDetail",
		AccountBasedExpenseLineDetail: AccountBasedExpenseLineDetail{
			AccountRef:  ReferenceType{NameValue: NameValue{Value: "64", Name: "Decks and Patios"}},
			CustomerRef: &ReferenceType{NameValue: NameValue{Value: "26"}},
		},
	}, lines[0])
	assert.Equal(t, "Debit", lines[1].JournalEntryLineDetail.PostingType)
	assert.Equal(t, "84", lines[1].JournalEntryLineDetail.AccountRef.Value)
	assert.Equal(t, &EntityTypeRef{Type: "Customer", EntityRef: ReferenceType{NameValue: NameValue{Value: "1"}}}, lines[1].JournalEntryLineDetail.Entity)
	require.NotNil(t, lines[2].GroupLineDetail)
	require.Len(t, lines[2].GroupLineDetail.Line, 1)
	assert.Equal(t, json.Number("1"), lines[2].GroupLineDetail.Line[0].SalesItemLineDetail.Qty)
}
