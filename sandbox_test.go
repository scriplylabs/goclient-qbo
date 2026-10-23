//go:build sandbox

// Tests against a real QuickBooks sandbox company, to catch QuickBooks
// changing under the library (and under qbotest, which copies what these
// tests check). They create and then delete transactions in the sandbox.
//
// Run with:
//
//	go test -tags sandbox -run Sandbox -v .
//
// Credentials come from token.txt in the repo root (gitignored): paste the
// OAuth Playground's "Get tokens" response, or a bare access token, and add a
// line realm=<company ID>, or set QBO_SANDBOX_REALM. The token needs the
// com.intuit.quickbooks.accounting scope and lasts an hour.
package quickbooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sandboxNote = "goclient-qbo sandbox test"

func newSandboxClient(t *testing.T) *Client {
	t.Helper()

	raw, err := os.ReadFile("token.txt")
	require.NoError(t, err, "the sandbox tests need token.txt in the repo root")

	token := ""
	if m := regexp.MustCompile(`"(?:accessToken|access_token)"\s*:\s*"([^"]+)"`).FindSubmatch(raw); m != nil {
		token = string(m[1])
	}

	realm := os.Getenv("QBO_SANDBOX_REALM")
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "realm="); ok && realm == "" {
			realm = v
		} else if token == "" && line != "" && !strings.ContainsAny(line, `"{}:= `) {
			token = line
		}
	}

	require.NotEmpty(t, token, "token.txt has no access token")
	require.NotEmpty(t, realm, "set realm=<company ID> in token.txt or QBO_SANDBOX_REALM")

	c, err := NewClientWithEndpoint(SandboxEndpoint, realm, "", &BearerToken{AccessToken: token})
	require.NoError(t, err)

	return c
}

// sandboxRefs are accounts and contacts looked up in the sandbox company.
type sandboxRefs struct {
	expense, income, bank, ar, ap, customer, vendor, item ReferenceType
}

func lookUpSandboxRefs(t *testing.T, c *Client) sandboxRefs {
	t.Helper()

	first := func(entity, where string) ReferenceType {
		var resp struct {
			QueryResponse map[string]json.RawMessage
		}
		require.NoError(t, c.query("SELECT * FROM "+entity+" "+where+" MAXRESULTS 1", &resp))

		var found []struct {
			ID string `json:"Id"`
		}
		require.NoError(t, json.Unmarshal(resp.QueryResponse[entity], &found))
		require.NotEmpty(t, found, "the sandbox has no %s %s", entity, where)

		return ReferenceType{NameValue: NameValue{Value: found[0].ID}}
	}

	return sandboxRefs{
		expense:  first("Account", "WHERE AccountType = 'Expense'"),
		income:   first("Account", "WHERE AccountType = 'Income'"),
		bank:     first("Account", "WHERE AccountType = 'Bank'"),
		ar:       first("Account", "WHERE AccountType = 'Accounts Receivable'"),
		ap:       first("Account", "WHERE AccountType = 'Accounts Payable'"),
		customer: first("Customer", ""),
		vendor:   first("Vendor", ""),
		item:     first("Item", "WHERE Type = 'Service'"),
	}
}

func newTodayDate() *Date {
	return &Date{time.Now()}
}

func newJELine(amount json.Number, posting string, account ReferenceType, entity *EntityTypeRef) Line {
	return Line{Amount: amount, DetailType: "JournalEntryLineDetail", JournalEntryLineDetail: JournalEntryLineDetail{
		PostingType: posting, AccountRef: account, Entity: entity,
	}}
}

// createJE creates a journal entry and deletes it when the test ends.
func createJE(t *testing.T, c *Client, input *JournalEntryCreateInput, opts ...RequestOption) (*JournalEntry, error) {
	t.Helper()

	note := sandboxNote
	input.PrivateNote = &note

	je, err := c.CreateJournalEntry(input, opts...)
	if err == nil {
		t.Cleanup(func() { assert.NoError(t, c.DeleteJournalEntry(je)) })
	}

	return je, err
}

// assertAmount compares an amount by value; QuickBooks writes 5 as 5.00.
func assertAmount(t *testing.T, want float64, got json.Number) {
	t.Helper()

	f, err := got.Float64()
	require.NoError(t, err)
	assert.Equal(t, want, f)
}

// requireFault checks that err is a Failure of the given type and code.
func requireFault(t *testing.T, err error, faultType string, code string) Failure {
	t.Helper()

	var failure Failure
	require.True(t, errors.As(err, &failure), "want a Failure, got %T: %v", err, err)
	assert.Equal(t, faultType, failure.Fault.Type)
	require.NotEmpty(t, failure.Fault.Error)
	assert.Equal(t, code, failure.Fault.Error[0].Code, failure.Fault.Error[0].Detail)
	assert.False(t, failure.Time.IsZero(), "fault time not decoded")

	return failure
}

func TestSandboxJournalEntry(t *testing.T) {
	c := newSandboxClient(t)
	refs := lookUpSandboxRefs(t, c)
	today := newTodayDate()

	je, err := createJE(t, c, &JournalEntryCreateInput{TxnDate: today, Line: []Line{
		newJELine("10.00", "Debit", refs.expense, nil),
		newJELine("10.00", "Credit", refs.bank, nil),
	}})
	require.NoError(t, err)

	got, err := c.FindJournalEntryByID(je.ID)
	require.NoError(t, err)
	assert.Equal(t, today.Format("2006-01-02"), got.TxnDate.Format("2006-01-02"))
	require.Len(t, got.Line, 2)
	assert.Equal(t, refs.expense.Value, got.Line[0].JournalEntryLineDetail.AccountRef.Value)
	assert.Equal(t, "Credit", got.Line[1].JournalEntryLineDetail.PostingType)
	assert.False(t, got.MetaData.CreateTime.IsZero())
}

// QuickBooks takes the date part of a TxnDate as written, ignoring a time and offset.
func TestSandboxTxnDateIgnoresTime(t *testing.T) {
	c := newSandboxClient(t)
	refs := lookUpSandboxRefs(t, c)
	day := time.Now().Format("2006-01-02")

	for _, suffix := range []string{"T02:00:00Z", "T23:30:00-07:00", "T00:00:00+14:00"} {
		t.Run(suffix, func(t *testing.T) {
			payload := fmt.Sprintf(`{"TxnDate":%q,"PrivateNote":%q,"Line":[
				{"Amount":1.00,"DetailType":"JournalEntryLineDetail","JournalEntryLineDetail":{"PostingType":"Debit","AccountRef":{"value":%q}}},
				{"Amount":1.00,"DetailType":"JournalEntryLineDetail","JournalEntryLineDetail":{"PostingType":"Credit","AccountRef":{"value":%q}}}]}`,
				day+suffix, sandboxNote, refs.expense.Value, refs.bank.Value)

			var resp struct{ JournalEntry JournalEntry }
			require.NoError(t, c.post("journalentry", json.RawMessage(payload), &resp, nil))
			t.Cleanup(func() { assert.NoError(t, c.DeleteJournalEntry(&resp.JournalEntry)) })

			assert.Equal(t, day, resp.JournalEntry.TxnDate.Format("2006-01-02"))
		})
	}
}

func TestSandboxRequestID(t *testing.T) {
	c := newSandboxClient(t)
	refs := lookUpSandboxRefs(t, c)
	requestID := fmt.Sprintf("goclient-qbo-%d", time.Now().UnixNano())
	entry := func(amount json.Number) *JournalEntryCreateInput {
		return &JournalEntryCreateInput{TxnDate: newTodayDate(), Line: []Line{
			newJELine(amount, "Debit", refs.expense, nil), newJELine(amount, "Credit", refs.bank, nil),
		}}
	}

	first, err := createJE(t, c, entry("2.00"), WithRequestID(requestID))
	require.NoError(t, err)

	// A repeat returns the first entry, even with a different body.
	again, err := c.CreateJournalEntry(entry("3.00"), WithRequestID(requestID))
	require.NoError(t, err)
	assert.Equal(t, first.ID, again.ID)
	assertAmount(t, 2, again.Line[0].Amount)

	_, err = createJE(t, c, entry("1.00"), WithRequestID((requestID + strings.Repeat("x", 50))[:50]))
	require.NoError(t, err, "a 50-character requestid is allowed")

	_, err = createJE(t, c, entry("1.00"), WithRequestID(strings.Repeat("y", 51)))
	failure := requireFault(t, err, "ValidationFault", "6000")
	assert.Contains(t, failure.Fault.Error[0].Detail, "50 characters")
}

func TestSandboxContactLines(t *testing.T) {
	c := newSandboxClient(t)
	refs := lookUpSandboxRefs(t, c)

	_, err := createJE(t, c, &JournalEntryCreateInput{TxnDate: newTodayDate(), Line: []Line{
		newJELine("4.00", "Debit", refs.ar, &EntityTypeRef{Type: "Customer", EntityRef: refs.customer}),
		newJELine("4.00", "Credit", refs.bank, nil),
		newJELine("6.00", "Debit", refs.bank, nil),
		newJELine("6.00", "Credit", refs.ap, &EntityTypeRef{Type: "Vendor", EntityRef: refs.vendor}),
	}})
	require.NoError(t, err, "A/R and A/P lines that name the contact")

	_, err = createJE(t, c, &JournalEntryCreateInput{TxnDate: newTodayDate(), Line: []Line{
		newJELine("4.00", "Debit", refs.ar, nil), newJELine("4.00", "Credit", refs.bank, nil),
	}})
	requireFault(t, err, "ValidationFault", "6000")

	_, err = createJE(t, c, &JournalEntryCreateInput{TxnDate: newTodayDate(), Line: []Line{
		newJELine("4.00", "Debit", refs.expense, nil), newJELine("5.00", "Credit", refs.bank, nil),
	}})
	requireFault(t, err, "ValidationFault", "2300")
}

func TestSandboxPaymentAndBillPayment(t *testing.T) {
	c := newSandboxClient(t)
	refs := lookUpSandboxRefs(t, c)
	note := sandboxNote

	payment, err := c.CreatePayment(&PaymentCreateInput{
		CustomerRef: refs.customer, TotalAmt: "5.00", TxnDate: newTodayDate(), DepositToAccountRef: &refs.bank,
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, c.DeletePayment(payment)) })
	assert.Equal(t, refs.bank.Value, payment.DepositToAccountRef.Value)
	assertAmount(t, 5, payment.UnappliedAmt)

	bill, err := c.CreateBill(&BillCreateInput{VendorRef: refs.vendor, TxnDate: newTodayDate(), PrivateNote: &note, Line: []Line{{
		Amount: "5.00", DetailType: "AccountBasedExpenseLineDetail",
		AccountBasedExpenseLineDetail: AccountBasedExpenseLineDetail{AccountRef: refs.expense},
	}}})
	require.NoError(t, err)

	billPayment, err := c.CreateBillPayment(&BillPaymentCreateInput{
		VendorRef: refs.vendor, PayType: "Check", TotalAmt: "5.00", TxnDate: newTodayDate(), PrivateNote: &note,
		CheckPayment: &BillPaymentCheckPayment{BankAccountRef: refs.bank},
		Line:         []PaymentLine{{Amount: "5.00", LinkedTxn: []LinkedTxn{{TxnID: bill.ID, TxnType: "Bill"}}}},
	})
	require.NoError(t, err)
	// Cleanups run last-in first-out: the bill payment goes before its bill.
	t.Cleanup(func() { assert.NoError(t, c.DeleteBill(bill)) })
	t.Cleanup(func() { assert.NoError(t, c.DeleteBillPayment(billPayment)) })
	assert.Equal(t, "Check", billPayment.PayType)

	_, err = c.CreateBillPayment(&BillPaymentCreateInput{
		VendorRef: refs.vendor, PayType: "Check", TotalAmt: "5.00", TxnDate: newTodayDate(),
		CheckPayment: &BillPaymentCheckPayment{BankAccountRef: refs.bank},
	})
	requireFault(t, err, "ValidationFault", "2020")
}

func TestSandboxFaults(t *testing.T) {
	c := newSandboxClient(t)

	t.Run("bad token", func(t *testing.T) {
		bad, err := NewClientWithEndpoint(SandboxEndpoint, c.realm, "", &BearerToken{AccessToken: "not-a-real-token"})
		require.NoError(t, err)

		_, err = bad.FindPreferences()
		requireFault(t, err, "AUTHENTICATION", "3200")
	})

	t.Run("account period closed", func(t *testing.T) {
		var resp struct {
			Preferences struct {
				AccountingInfoPrefs struct{ BookCloseDate *Date }
			}
		}
		require.NoError(t, c.get("preferences", &resp, nil))

		closed := resp.Preferences.AccountingInfoPrefs.BookCloseDate
		if closed == nil {
			t.Skip("the sandbox company has no book close date; set one to run this")
		}

		refs := lookUpSandboxRefs(t, c)
		_, err := createJE(t, c, &JournalEntryCreateInput{TxnDate: &Date{closed.AddDate(0, 0, -1)}, Line: []Line{
			newJELine("1.00", "Debit", refs.expense, nil), newJELine("1.00", "Credit", refs.bank, nil),
		}})
		requireFault(t, err, "ValidationFault", "6200")
	})
}

// GetChangedEntities reports an entity deleted in QuickBooks as Deleted.
func TestSandboxChangedEntitiesDeleted(t *testing.T) {
	c := newSandboxClient(t)
	refs := lookUpSandboxRefs(t, c)
	since := time.Now().Add(-time.Minute)

	payment, err := c.CreatePayment(&PaymentCreateInput{
		CustomerRef: refs.customer, TotalAmt: "1.00", TxnDate: newTodayDate(), DepositToAccountRef: &refs.bank,
	})
	require.NoError(t, err)
	require.NoError(t, c.DeletePayment(payment))

	var deleted *DeletedEntity
	for attempt := 0; attempt < 10 && deleted == nil; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Second)
		}

		changes, err := c.GetChangedEntities([]string{"Payment"}, since)
		require.NoError(t, err)

		for _, p := range changes.Payments {
			if p.Deleted != nil && p.Deleted.ID == payment.ID {
				deleted = p.Deleted
			}
			if p.Entity != nil && p.Entity.ID == payment.ID {
				assert.Fail(t, "deleted payment came back as a live entity")
			}
		}
	}

	require.NotNil(t, deleted, "deleted payment %s not in the change feed", payment.ID)
}

// Payment, Deposit, CreditMemo and Estimate keep the PrivateNote they are
// created with, and a deposit banks a payment and takes a direct line.
func TestSandboxPrivateNote(t *testing.T) {
	c := newSandboxClient(t)
	refs := lookUpSandboxRefs(t, c)
	note := sandboxNote
	salesLines := []Line{{Amount: "5.00", DetailType: "SalesItemLineDetail", SalesItemLineDetail: SalesItemLineDetail{ItemRef: &refs.item}}}

	// With no DepositToAccountRef the payment waits in Undeposited Funds,
	// for the deposit to bank it.
	payment, err := c.CreatePayment(&PaymentCreateInput{CustomerRef: refs.customer, TotalAmt: "5.00", TxnDate: newTodayDate(), PrivateNote: &note})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, c.DeletePayment(payment)) })
	found, err := c.FindPaymentByID(payment.ID)
	require.NoError(t, err)
	assert.Equal(t, &note, found.PrivateNote)

	zero := "0"
	deposit, err := c.CreateDeposit(&DepositCreateInput{
		DepositToAccountRef: refs.bank, TxnDate: newTodayDate(), PrivateNote: &note,
		Line: []DepositLine{
			{Amount: "5.00", LinkedTxn: []LinkedTxn{{TxnID: payment.ID, TxnType: "Payment", TxnLineID: &zero}}},
			{Amount: "7.00", DetailType: "DepositLineDetail", DepositLineDetail: &DepositLineDetail{AccountRef: refs.income}},
		},
	})
	require.NoError(t, err)
	// Cleanups run last-in first-out: the deposit goes before its payment.
	t.Cleanup(func() { assert.NoError(t, c.DeleteDeposit(deposit)) })
	foundDeposit, err := c.FindDepositByID(deposit.ID)
	require.NoError(t, err)
	assert.Equal(t, &note, foundDeposit.PrivateNote)
	assertAmount(t, 12, foundDeposit.TotalAmt)
	require.Len(t, foundDeposit.Line, 2)
	assert.Equal(t, payment.ID, foundDeposit.Line[0].LinkedTxn[0].TxnID)
	require.NotNil(t, foundDeposit.Line[1].DepositLineDetail)
	assert.Equal(t, refs.income.Value, foundDeposit.Line[1].DepositLineDetail.AccountRef.Value)

	creditMemo, err := c.CreateCreditMemo(&CreditMemoCreateInput{CustomerRef: refs.customer, TxnDate: newTodayDate(), PrivateNote: &note, Line: salesLines})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, c.DeleteCreditMemo(creditMemo)) })
	foundCreditMemo, err := c.FindCreditMemoByID(creditMemo.ID)
	require.NoError(t, err)
	assert.Equal(t, &note, foundCreditMemo.PrivateNote)

	estimate, err := c.CreateEstimate(&EstimateCreateInput{CustomerRef: refs.customer, TxnDate: newTodayDate(), PrivateNote: &note, Line: salesLines})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, c.DeleteEstimate(estimate)) })
	foundEstimate, err := c.FindEstimateByID(estimate.ID)
	require.NoError(t, err)
	assert.Equal(t, &note, foundEstimate.PrivateNote)
}
