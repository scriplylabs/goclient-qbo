package qbotest_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	quickbooks "github.com/scriplylabs/goclient-qbo"
	"github.com/scriplylabs/goclient-qbo/qbotest"
)

func ref(v string) quickbooks.ReferenceType {
	return quickbooks.ReferenceType{NameValue: quickbooks.NameValue{Value: v}}
}

func journalEntry() *quickbooks.JournalEntryCreateInput {
	return &quickbooks.JournalEntryCreateInput{
		TxnDate: &quickbooks.Date{Time: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		Line: []quickbooks.Line{
			{Amount: "10.00", DetailType: "JournalEntryLineDetail", JournalEntryLineDetail: quickbooks.JournalEntryLineDetail{PostingType: "Debit", AccountRef: ref("79")}},
			{Amount: "10.00", DetailType: "JournalEntryLineDetail", JournalEntryLineDetail: quickbooks.JournalEntryLineDetail{PostingType: "Credit", AccountRef: ref("35")}},
		},
	}
}

func TestCreatesRecordBodyAndAnswerWithID(t *testing.T) {
	srv, client := qbotest.NewServer(t)

	je, err := client.CreateJournalEntry(journalEntry())
	require.NoError(t, err)
	assert.NotEmpty(t, je.ID)
	assert.Equal(t, "0", je.SyncToken)
	assert.Equal(t, "2026-10-01", je.TxnDate.Format("2006-01-02"))
	assert.Equal(t, json.Number("10.00"), je.Line[0].Amount)

	payment, err := client.CreatePayment(&quickbooks.PaymentCreateInput{CustomerRef: ref("1"), TotalAmt: "5.00", DepositToAccountRef: &[]quickbooks.ReferenceType{ref("36")}[0]})
	require.NoError(t, err)

	billPayment, err := client.CreateBillPayment(&quickbooks.BillPaymentCreateInput{
		VendorRef: ref("56"), PayType: "Check", TotalAmt: "5.00",
		CheckPayment: &quickbooks.BillPaymentCheckPayment{BankAccountRef: ref("36")},
	})
	require.NoError(t, err)
	assert.NotEqual(t, payment.ID, billPayment.ID)

	requests := srv.RequestsTo("journalentry")
	require.Len(t, requests, 1)
	assert.Equal(t, "POST", requests[0].Method)
	assert.Equal(t, 200, requests[0].Status)
	assert.Equal(t, `{"Line":[`+
		`{"Amount":10.00,"DetailType":"JournalEntryLineDetail","JournalEntryLineDetail":{"PostingType":"Debit","AccountRef":{"value":"79"}}},`+
		`{"Amount":10.00,"DetailType":"JournalEntryLineDetail","JournalEntryLineDetail":{"PostingType":"Credit","AccountRef":{"value":"35"}}}],`+
		`"TxnDate":"2026-10-01"}`, string(requests[0].Body))

	assert.JSONEq(t, `{"CustomerRef":{"value":"1"},"TotalAmt":5.00,"DepositToAccountRef":{"value":"36"}}`, string(srv.RequestsTo("payment")[0].Body))
	assert.Len(t, srv.Entities("JournalEntry"), 1)

	found, err := client.FindJournalEntryByID(je.ID)
	require.NoError(t, err)
	assert.Equal(t, je.ID, found.ID)
}

func TestRepeatedRequestIDReturnsFirstAnswer(t *testing.T) {
	srv, client := qbotest.NewServer(t)

	first, err := client.CreateJournalEntry(journalEntry(), quickbooks.WithRequestID("step-1"))
	require.NoError(t, err)

	different := journalEntry()
	different.Line[0].Amount, different.Line[1].Amount = "99.00", "99.00"
	again, err := client.CreateJournalEntry(different, quickbooks.WithRequestID("step-1"))
	require.NoError(t, err)
	assert.Equal(t, first.ID, again.ID)
	assert.Equal(t, json.Number("10.00"), again.Line[0].Amount)

	other, err := client.CreateJournalEntry(journalEntry(), quickbooks.WithRequestID("step-2"))
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, other.ID)

	assert.Len(t, srv.Entities("JournalEntry"), 2)
	assert.Equal(t, "step-1", srv.RequestsTo("journalentry")[1].RequestID())
}

// A failed create isn't remembered, so a retry with the same requestid goes through.
func TestRequestIDAfterFault(t *testing.T) {
	srv, client := qbotest.NewServer(t)
	srv.FailNext("journalentry", qbotest.ServerError())

	_, err := client.CreateJournalEntry(journalEntry(), quickbooks.WithRequestID("step-1"))
	require.Error(t, err)

	je, err := client.CreateJournalEntry(journalEntry(), quickbooks.WithRequestID("step-1"))
	require.NoError(t, err)
	assert.NotEmpty(t, je.ID)
}

func TestFaults(t *testing.T) {
	tests := []struct {
		name     string
		fault    qbotest.Fault
		status   int
		wantType string
		wantCode string
	}{
		{"account period closed", qbotest.AccountPeriodClosed(), 400, "ValidationFault", "6200"},
		{"validation fault", qbotest.ValidationFault("6000", "A business validation error has occurred while processing your request", "Business Validation Error: When you use Accounts Receivable, you must choose a customer in the Name field."), 400, "ValidationFault", "6000"},
		{"authentication", qbotest.AuthenticationFault(), 401, "AUTHENTICATION", "3200"},
		{"server error", qbotest.ServerError(), 500, "SystemFault", "10000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, client := qbotest.NewServer(t)
			srv.FailNext("journalentry", tt.fault)

			_, err := client.CreateJournalEntry(journalEntry())

			var failure quickbooks.Failure
			require.True(t, errors.As(err, &failure), "want a Failure, got %T: %v", err, err)
			assert.Equal(t, tt.wantType, failure.Fault.Type)
			assert.Equal(t, tt.wantCode, failure.Fault.Error[0].Code)
			assert.False(t, failure.Time.IsZero())
			assert.Equal(t, tt.status, srv.RequestsTo("journalentry")[0].Status)
			assert.Empty(t, srv.Entities("JournalEntry"))

			// The fault is used up: the next request succeeds.
			_, err = client.CreateJournalEntry(journalEntry())
			require.NoError(t, err)
		})
	}
}

func TestFailNextMatchesPath(t *testing.T) {
	srv, client := qbotest.NewServer(t)
	srv.FailNext("payment", qbotest.ServerError())

	_, err := client.CreateJournalEntry(journalEntry())
	require.NoError(t, err)

	_, err = client.CreatePayment(&quickbooks.PaymentCreateInput{CustomerRef: ref("1"), TotalAmt: "1"})
	require.Error(t, err)
}

func TestHangPastClientTimeout(t *testing.T) {
	srv, client := qbotest.NewServer(t)
	client.Client.Timeout = 100 * time.Millisecond
	srv.FailNext("", qbotest.Hang())

	start := time.Now()
	_, err := client.CreateJournalEntry(journalEntry())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Client.Timeout")
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Equal(t, 0, srv.RequestsTo("journalentry")[0].Status)
}

func TestAccountsCreateAndQuery(t *testing.T) {
	srv, client := qbotest.NewServer(t)
	clearingID := srv.Add("Account", quickbooks.Account{Name: "Scriply Clearing", AccountType: quickbooks.BankAccountType})

	inactive := false
	created, err := client.CreateAccount(&quickbooks.AccountCreateInput{Name: "Old", AccountType: quickbooks.ExpenseAccountType, Active: &inactive})
	require.NoError(t, err)

	accounts, err := client.FindAccounts()
	require.NoError(t, err)
	require.Len(t, accounts, 1, "inactive accounts are left out")
	assert.Equal(t, clearingID, accounts[0].ID)

	page, err := client.ListAccounts("", 10)
	require.NoError(t, err)
	assert.Len(t, page.Items, 2, "ListAccounts asks for inactive ones too")
	assert.Empty(t, page.NextPageToken)

	byName, err := client.QueryAccounts("SELECT * FROM Account WHERE Name = 'Scriply Clearing' AND AccountType = 'Bank'")
	require.NoError(t, err)
	require.Len(t, byName, 1)
	assert.Equal(t, clearingID, byName[0].ID)

	_, err = client.QueryAccounts("SELECT * FROM Account WHERE Name LIKE 'Scriply%'")
	var failure quickbooks.Failure
	require.True(t, errors.As(err, &failure), "an unsupported query is a fault, not an empty result")

	desc := "renamed"
	created.Description = &desc
	updated, err := client.UpdateAccount(created)
	require.NoError(t, err)
	assert.Equal(t, "1", updated.SyncToken)
	assert.Equal(t, "Old", updated.Name, "a sparse update keeps fields it doesn't send")
	assert.Equal(t, "renamed", *updated.Description)
}

func TestPaginationAndCount(t *testing.T) {
	srv, client := qbotest.NewServer(t)
	for i := 0; i < 5; i++ {
		srv.Add("Vendor", map[string]any{"DisplayName": "Vendor", "Active": true})
	}

	vendors, err := client.FindVendors()
	require.NoError(t, err)
	assert.Len(t, vendors, 5)

	page, err := client.ListVendors("", 2)
	require.NoError(t, err)
	assert.Len(t, page.Items, 2)
	assert.Equal(t, "3", page.NextPageToken)
}

func TestPreferencesAndCompanyInfo(t *testing.T) {
	_, client := qbotest.NewServer(t)

	prefs, err := client.FindPreferences()
	require.NoError(t, err)
	assert.Equal(t, "USD", prefs.CurrencyPrefs.HomeCurrency.Value)

	info, err := client.FindCompanyInfo()
	require.NoError(t, err)
	assert.Equal(t, "qbotest company", info.CompanyName)
}

func TestChangedEntities(t *testing.T) {
	_, client := qbotest.NewServer(t)
	since := time.Now().Add(-time.Minute)

	kept, err := client.CreatePayment(&quickbooks.PaymentCreateInput{CustomerRef: ref("1"), TotalAmt: "1"})
	require.NoError(t, err)
	gone, err := client.CreatePayment(&quickbooks.PaymentCreateInput{CustomerRef: ref("1"), TotalAmt: "2"})
	require.NoError(t, err)
	require.NoError(t, client.DeletePayment(gone))

	changes, err := client.GetChangedEntities([]string{"Payment", "Customer"}, since)
	require.NoError(t, err)
	require.Len(t, changes.Payments, 2)
	assert.Equal(t, kept.ID, changes.Payments[0].Entity.ID)
	require.NotNil(t, changes.Payments[1].Deleted)
	assert.Equal(t, gone.ID, changes.Payments[1].Deleted.ID)
	assert.Empty(t, changes.Customers)

	_, err = client.FindPaymentByID(gone.ID)
	require.Error(t, err)
}
