package main

import (
	"encoding/json"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

func createTestBook(t *testing.T, app core.App, id, sellerId, eventId string, price float64, status string) *core.Record {
	booksColl, _ := app.FindCollectionByNameOrId("books")
	b := core.NewRecord(booksColl)
	if id != "" {
		b.Id = id
	}
	b.Set("seller", sellerId)
	b.Set("event", eventId)
	b.Set("price", price)
	b.Set("status", status)
	b.Set("accepted", true)
	coverBytes, fileName, _ := generateDummyCoverBytes("Test Title", "Author", color.RGBA{R: 50, G: 100, B: 150, A: 255})
	file, _ := filesystem.NewFileFromBytes(coverBytes, fileName)
	b.Set("photo", file)
	if err := app.Save(b); err != nil {
		t.Fatalf("Failed to create test book: %v", err)
	}
	return b
}

func TestMoneyReturnsSchema(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()
	_ = ensureSchema(testApp)

	coll, err := testApp.FindCollectionByNameOrId("money_returns")
	if err != nil || coll == nil {
		t.Fatalf("Collection money_returns not found: %v", err)
	}

	requiredFields := []string{"seller", "event", "amount", "method", "fio_transaction_id", "variableSymbol", "cashier", "created", "updated"}
	for _, f := range requiredFields {
		if coll.Fields.GetByName(f) == nil {
			t.Errorf("Missing field in money_returns: %s", f)
		}
	}

	// Verify status field in books has "returned"
	booksColl, err := testApp.FindCollectionByNameOrId("books")
	if err != nil {
		t.Fatalf("Books collection not found: %v", err)
	}
	statusField, ok := booksColl.Fields.GetByName("status").(*core.SelectField)
	if !ok {
		t.Fatalf("Books status field is not SelectField")
	}
	hasReturned := false
	for _, v := range statusField.Values {
		if v == "returned" {
			hasReturned = true
			break
		}
	}
	if !hasReturned {
		t.Errorf("Books status field does not contain 'returned' option: %v", statusField.Values)
	}
}

func TestDynamicSellerBalancesAndCashPayout(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()
	_ = ensureSchema(testApp)
	_ = seedInitialData(testApp)

	event, err := testApp.FindFirstRecordByData("events", "active", true)
	if err != nil || event == nil {
		t.Fatalf("Active event not found: %v", err)
	}

	seller, err := testApp.FindAuthRecordByEmail("users", "seller@burza.cz")
	if err != nil || seller == nil {
		t.Fatalf("Seller not found: %v", err)
	}

	// Remove default seeded sample books to test exact amounts
	sampleBooks, _ := testApp.FindAllRecords("books", nil)
	for _, sb := range sampleBooks {
		_ = testApp.Delete(sb)
	}

	// Create 2 sold books for seller: 300 Kč and 250 Kč (total 550 Kč)
	createTestBook(t, testApp, "soldbook0000001", seller.Id, event.Id, 300.0, "bought")
	createTestBook(t, testApp, "soldbook0000002", seller.Id, event.Id, 250.0, "bought")

	// Create 1 unsold book (available): 200 Kč
	createTestBook(t, testApp, "unsoldbook00001", seller.Id, event.Id, 200.0, "available")

	// Initial balance: 550 Kč earned, 0 Kč paid, 550 Kč unpaid
	bal, err := CalculateSellerBalance(testApp, seller.Id, event.Id)
	if err != nil {
		t.Fatalf("CalculateSellerBalance failed: %v", err)
	}
	if bal.TotalEarned != 550.0 {
		t.Errorf("Expected TotalEarned 550, got %f", bal.TotalEarned)
	}
	if bal.TotalPaid != 0.0 {
		t.Errorf("Expected TotalPaid 0, got %f", bal.TotalPaid)
	}
	if bal.TotalUnpaid != 550.0 {
		t.Errorf("Expected TotalUnpaid 550, got %f", bal.TotalUnpaid)
	}
	if bal.SoldCount != 2 {
		t.Errorf("Expected SoldCount 2, got %d", bal.SoldCount)
	}
	if bal.UnsoldCount != 1 {
		t.Errorf("Expected UnsoldCount 1, got %d", bal.UnsoldCount)
	}

	// Add partial cash payout of 200 Kč
	returnsColl, _ := testApp.FindCollectionByNameOrId("money_returns")
	r1 := core.NewRecord(returnsColl)
	r1.Set("seller", seller.Id)
	r1.Set("event", event.Id)
	r1.Set("amount", 200.0)
	r1.Set("method", "cash")
	if err := testApp.Save(r1); err != nil {
		t.Fatalf("Failed to save money_returns record: %v", err)
	}

	// Re-check balance: 550 earned, 200 paid, 350 unpaid
	bal, err = CalculateSellerBalance(testApp, seller.Id, event.Id)
	if err != nil {
		t.Fatalf("CalculateSellerBalance failed: %v", err)
	}
	if bal.TotalEarned != 550.0 {
		t.Errorf("Expected TotalEarned 550, got %f", bal.TotalEarned)
	}
	if bal.TotalPaid != 200.0 {
		t.Errorf("Expected TotalPaid 200, got %f", bal.TotalPaid)
	}
	if bal.TotalUnpaid != 350.0 {
		t.Errorf("Expected TotalUnpaid 350, got %f", bal.TotalUnpaid)
	}
	if bal.PaidInCash != 200.0 {
		t.Errorf("Expected PaidInCash 200, got %f", bal.PaidInCash)
	}
	if bal.PaidInBank != 0.0 {
		t.Errorf("Expected PaidInBank 0, got %f", bal.PaidInBank)
	}
}

func TestNormalizeCzechAccount(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"2101234567", "2101234567"},
		{"002101234567", "2101234567"},
		{"19-12345678", "19-12345678"},
		{"000019-0012345678", "19-12345678"},
		{"000000-0012345678", "12345678"},
		{"  19 - 12345678  ", "19-12345678"},
		{"0", "0"},
		{"0000", "0"},
	}

	for _, tc := range cases {
		got := normalizeCzechAccount(tc.input)
		if got != tc.expected {
			t.Errorf("normalizeCzechAccount(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestIbanToCzechAccount(t *testing.T) {
	testsCases := []struct {
		input       string
		expectedAcc string
		expectedBnk string
		shouldErr   bool
	}{
		{"CZ6520100000002101234567", "2101234567", "2010", false},
		{"CZ8201000000191234567890", "19-1234567890", "0100", false},
		{"CZ8201000000190012345678", "19-12345678", "0100", false},
		{"2101234567/2010", "2101234567", "2010", false},
		{"19-1234567890/0100", "19-1234567890", "0100", false},
		{"000019-0012345678/0100", "19-12345678", "0100", false},
		{"invalid", "", "", true},
	}

	for _, tc := range testsCases {
		acc, bnk, err := ibanToCzechAccount(tc.input)
		if tc.shouldErr {
			if err == nil {
				t.Errorf("Expected error for %s, got none", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.input, err)
			}
			if acc != tc.expectedAcc {
				t.Errorf("For %s, expected accountTo %s, got %s", tc.input, tc.expectedAcc, acc)
			}
			if bnk != tc.expectedBnk {
				t.Errorf("For %s, expected bankCode %s, got %s", tc.input, tc.expectedBnk, bnk)
			}
		}
	}
}

func TestFioXmlGeneration(t *testing.T) {
	summaries := []*SellerBalanceSummary{
		{
			SellerId:     "seller123456789",
			Name:         "Jan Novák & syn",
			Email:        "jan@example.com",
			PayoutToBank: true,
			Iban:         "CZ6520100000002101234567",
			TotalEarned:  600.0,
			TotalPaid:    200.0,
			TotalUnpaid:  400.0,
		},
		{
			SellerId:     "cashSeller99999",
			Name:         "Hotovostní Prodejce",
			Email:        "cash@example.com",
			PayoutToBank: false, // Should be omitted from Fio XML!
			Iban:         "",
			TotalUnpaid:  300.0,
		},
	}

	date := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	xmlBytes, err := GenerateFioXmlBatch("2101234567/2010", summaries, date)
	if err != nil {
		t.Fatalf("GenerateFioXmlBatch failed: %v", err)
	}

	xmlStr := string(xmlBytes)

	// Check header
	if !strings.Contains(xmlStr, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("Missing XML declaration")
	}
	if !strings.Contains(xmlStr, `http://www.fio.cz/schema/importIB.xsd`) {
		t.Errorf("Missing Fio schema location")
	}

	// Check that only 1 DomesticTransaction exists (cash seller omitted)
	count := strings.Count(xmlStr, "<DomesticTransaction>")
	if count != 1 {
		t.Errorf("Expected 1 DomesticTransaction, got %d", count)
	}

	// Check exact fields and ordering for seller 1
	if !strings.Contains(xmlStr, "<accountFrom>2101234567</accountFrom>") {
		t.Errorf("Expected accountFrom 2101234567")
	}
	if !strings.Contains(xmlStr, "<amount>400.00</amount>") {
		t.Errorf("Expected amount 400.00 (TotalUnpaid)")
	}
	if !strings.Contains(xmlStr, "<accountTo>2101234567</accountTo>") {
		t.Errorf("Expected accountTo 2101234567")
	}
	if !strings.Contains(xmlStr, "<bankCode>2010</bankCode>") {
		t.Errorf("Expected bankCode 2010")
	}
	if !strings.Contains(xmlStr, "<date>2026-09-13</date>") {
		t.Errorf("Expected date 2026-09-13")
	}
	// Check XML escaping of '&' in "Jan Novák & syn"
	if !strings.Contains(xmlStr, "Jan Nov&#225;k &amp; syn") && !strings.Contains(xmlStr, "Jan Novák &amp; syn") {
		t.Errorf("Expected XML escaped '&amp;' in name, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, "ID: seller123456789") {
		t.Errorf("Expected seller ID in comment tag")
	}
	if !strings.Contains(xmlStr, "<paymentType>431001</paymentType>") {
		t.Errorf("Expected paymentType 431001")
	}
}

func TestSyncFioPayoutsReconciliation(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()
	_ = ensureSchema(testApp)
	_ = seedInitialData(testApp)

	event, _ := testApp.FindFirstRecordByData("events", "active", true)
	seller, _ := testApp.FindAuthRecordByEmail("users", "seller@burza.cz")

	// Clean out seeded sample books
	sampleBooks, _ := testApp.FindAllRecords("books", nil)
	for _, sb := range sampleBooks {
		_ = testApp.Delete(sb)
	}

	// Set seller bank details
	seller.Set("payoutToBank", true)
	seller.Set("iban", "CZ6520100000002101234567")
	_ = testApp.Save(seller)

	// Create sold book for 400 Kč
	createTestBook(t, testApp, "soldbook0000003", seller.Id, event.Id, 400.0, "bought")

	// Mock Fio server with an outgoing transaction (-400 Kč) containing the seller ID in comment
	mockFio := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "transactions.json") {
			http.NotFound(w, r)
			return
		}

		resp := map[string]any{
			"accountStatement": map[string]any{
				"info": map[string]any{
					"accountId": "2101234567",
					"currency":  "CZK",
				},
				"transactionList": map[string]any{
					"transaction": []map[string]any{
						{
							"column0":  map[string]any{"value": "2026-09-13+0200"},
							"column1":  map[string]any{"value": -400.0}, // OUTGOING!
							"column2":  map[string]any{"value": "2101234567"},
							"column3":  map[string]any{"value": "2010"},
							"column5":  map[string]any{"value": "26090001"},
							"column14": map[string]any{"value": "CZK"},
							"column16": map[string]any{"value": "Burza ucebnic - Vyplata"},
							"column22": map[string]any{"value": "fio_payout_tx_1"},
							"column25": map[string]any{"value": "ID: " + seller.Id + " Prodejce Jan"},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockFio.Close()

	origURL := fioBaseURL
	fioBaseURL = mockFio.URL
	defer func() { fioBaseURL = origURL }()

	resetFioRateLimit()

	// Initial balance: 400 unpaid
	balBefore, _ := CalculateSellerBalance(testApp, seller.Id, event.Id)
	if balBefore.TotalUnpaid != 400.0 {
		t.Fatalf("Expected TotalUnpaid 400 before sync, got %f", balBefore.TotalUnpaid)
	}

	// 1. Run sync
	result, err := SyncFioPayouts(testApp, "test_token")
	if err != nil {
		t.Fatalf("SyncFioPayouts failed: %v", err)
	}
	if result.MatchedCount != 1 {
		t.Fatalf("Expected 1 matched payout, got %d", result.MatchedCount)
	}
	if result.MatchedPayouts[0].SellerId != seller.Id {
		t.Errorf("Expected matched seller %s, got %s", seller.Id, result.MatchedPayouts[0].SellerId)
	}

	// Verify balance after sync: TotalPaid = 400, TotalUnpaid = 0
	balAfter, _ := CalculateSellerBalance(testApp, seller.Id, event.Id)
	if balAfter.TotalPaid != 400.0 {
		t.Errorf("Expected TotalPaid 400 after sync, got %f", balAfter.TotalPaid)
	}
	if balAfter.TotalUnpaid != 0.0 {
		t.Errorf("Expected TotalUnpaid 0 after sync, got %f", balAfter.TotalUnpaid)
	}

	// Verify money_returns record in DB
	returns, err := testApp.FindAllRecords("money_returns", nil)
	if err != nil || len(returns) != 1 {
		t.Fatalf("Expected 1 record in money_returns, got %d", len(returns))
	}
	ret := returns[0]
	if ret.GetString("method") != "bank" {
		t.Errorf("Expected method bank, got %s", ret.GetString("method"))
	}
	if ret.GetString("fio_transaction_id") != "fio_payout_tx_1" {
		t.Errorf("Expected fio_transaction_id fio_payout_tx_1, got %s", ret.GetString("fio_transaction_id"))
	}
	if ret.GetFloat("amount") != 400.0 {
		t.Errorf("Expected amount 400, got %f", ret.GetFloat("amount"))
	}

	// 2. Run sync again -> Idempotency check: should NOT create duplicate record!
	resetFioRateLimit()
	result2, err := SyncFioPayouts(testApp, "test_token")
	if err != nil {
		t.Fatalf("Second sync failed: %v", err)
	}
	if result2.MatchedCount != 0 {
		t.Errorf("Expected 0 matched on second run, got %d", result2.MatchedCount)
	}

	returns2, _ := testApp.FindAllRecords("money_returns", nil)
	if len(returns2) != 1 {
		t.Errorf("Expected still 1 record in money_returns, got %d", len(returns2))
	}
}

func TestSyncFioPayoutsPrefixedAccountMatch(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()
	_ = ensureSchema(testApp)
	_ = seedInitialData(testApp)

	event, _ := testApp.FindFirstRecordByData("events", "active", true)
	seller, _ := testApp.FindAuthRecordByEmail("users", "seller@burza.cz")

	// Clean out seeded sample books
	sampleBooks, _ := testApp.FindAllRecords("books", nil)
	for _, sb := range sampleBooks {
		_ = testApp.Delete(sb)
	}

	// Seller has prefixed bank account with leading zeros in IBAN / string
	seller.Set("payoutToBank", true)
	seller.Set("iban", "000019-0012345678/0100")
	_ = testApp.Save(seller)

	// Create sold book for 350 Kč
	createTestBook(t, testApp, "prefixbk0000001", seller.Id, event.Id, 350.0, "bought")

	// Mock Fio server with an outgoing transaction (-350 Kč) without seller ID, matching by normalized account
	mockFio := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "transactions.json") {
			http.NotFound(w, r)
			return
		}

		resp := map[string]any{
			"accountStatement": map[string]any{
				"info": map[string]any{
					"accountId": "2101234567",
					"currency":  "CZK",
				},
				"transactionList": map[string]any{
					"transaction": []map[string]any{
						{
							"column0":  map[string]any{"value": "2026-09-13+0200"},
							"column1":  map[string]any{"value": -350.0},
							"column2":  map[string]any{"value": "19-12345678"}, // Fio normalized account format
							"column3":  map[string]any{"value": "0100"},
							"column5":  map[string]any{"value": "26090002"},
							"column14": map[string]any{"value": "CZK"},
							"column16": map[string]any{"value": "Vyplata z burzy"}, // NO seller ID in comment
							"column22": map[string]any{"value": "fio_payout_tx_prefix_1"},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockFio.Close()

	origURL := fioBaseURL
	fioBaseURL = mockFio.URL
	defer func() { fioBaseURL = origURL }()

	resetFioRateLimit()

	result, err := SyncFioPayouts(testApp, "test_token")
	if err != nil {
		t.Fatalf("SyncFioPayouts failed: %v", err)
	}
	if result.MatchedCount != 1 {
		t.Fatalf("Expected 1 matched payout by prefixed account, got %d", result.MatchedCount)
	}
	if result.MatchedPayouts[0].SellerId != seller.Id {
		t.Errorf("Expected matched seller %s, got %s", seller.Id, result.MatchedPayouts[0].SellerId)
	}

	balAfter, _ := CalculateSellerBalance(testApp, seller.Id, event.Id)
	if balAfter.TotalPaid != 350.0 || balAfter.TotalUnpaid != 0.0 {
		t.Errorf("Expected TotalPaid 350 and TotalUnpaid 0, got TotalPaid=%f, TotalUnpaid=%f", balAfter.TotalPaid, balAfter.TotalUnpaid)
	}
}

func TestBookReturnAndBuyPrevention(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()
	_ = ensureSchema(testApp)
	_ = seedInitialData(testApp)

	event, _ := testApp.FindFirstRecordByData("events", "active", true)
	seller, _ := testApp.FindAuthRecordByEmail("users", "seller@burza.cz")
	buyer, _ := testApp.FindAuthRecordByEmail("users", "buyer@burza.cz")

	book := createTestBook(t, testApp, "booktoreturn001", seller.Id, event.Id, 150.0, "available")

	// Cashier returns book via MarkReturned
	book.Set("status", "returned")
	if err := testApp.Save(book); err != nil {
		t.Fatalf("Failed to mark book returned: %v", err)
	}

	// Verify non-cashier cannot un-return book
	var scenarioNonCashierUnreturn tests.ApiScenario
	scenarioNonCashierUnreturn = tests.ApiScenario{
		Name:            "Non-cashier cannot change status from returned",
		Method:          http.MethodPatch,
		URL:             "/api/collections/books/records/booktoreturn001",
		Body:            strings.NewReader(`{"status": "available"}`),
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{"Pouze pokladní může změnit stav již vrácené knihy"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
			sel, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
			ev, _ := app.FindFirstRecordByData("events", "active", true)
			createTestBook(tb.(*testing.T), app, "booktoreturn001", sel.Id, ev.Id, 150.0, "returned")
			tok, _ := sel.NewAuthToken()
			scenarioNonCashierUnreturn.Headers = map[string]string{"Authorization": tok}
		},
	}
	scenarioNonCashierUnreturn.Test(t)

	// Verify buyer cannot checkout returned book
	var scenarioCheckoutReturned tests.ApiScenario
	scenarioCheckoutReturned = tests.ApiScenario{
		Name:            "Buyer cannot checkout returned book",
		Method:          http.MethodPost,
		URL:             "/api/checkout",
		Body:            strings.NewReader(`{"bookIds": ["booktoreturn001"]}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"byla již vrácena prodejci a nelze ji zakoupit"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
			sel, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
			by, _ := app.FindAuthRecordByEmail("users", "buyer@burza.cz")
			ev, _ := app.FindFirstRecordByData("events", "active", true)
			createTestBook(tb.(*testing.T), app, "booktoreturn001", sel.Id, ev.Id, 150.0, "returned")
			tok, _ := by.NewAuthToken()
			scenarioCheckoutReturned.Headers = map[string]string{"Authorization": tok}
		},
	}
	scenarioCheckoutReturned.Test(t)

	// Verify cashier POS prepare-checkout rejects returned book
	var scenarioPrepareCheckoutReturned tests.ApiScenario
	scenarioPrepareCheckoutReturned = tests.ApiScenario{
		Name:            "Prepare-checkout rejects returned book",
		Method:          http.MethodPost,
		URL:             "/api/cashier/prepare-checkout",
		Body:            strings.NewReader(`{"buyerId": "` + buyer.Id + `", "bookIds": ["booktoreturn001"]}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"byla již vrácena prodejci a nelze ji zakoupit"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
			sel, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
			csh, _ := app.FindAuthRecordByEmail("users", "cashier@burza.cz")
			ev, _ := app.FindFirstRecordByData("events", "active", true)
			createTestBook(tb.(*testing.T), app, "booktoreturn001", sel.Id, ev.Id, 150.0, "returned")
			tok, _ := csh.NewAuthToken()
			scenarioPrepareCheckoutReturned.Headers = map[string]string{"Authorization": tok}
		},
	}
	scenarioPrepareCheckoutReturned.Test(t)
}

func TestCashierReturnsEndpoints(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()
	_ = ensureSchema(testApp)
	_ = seedInitialData(testApp)

	event, _ := testApp.FindFirstRecordByData("events", "active", true)
	seller, _ := testApp.FindAuthRecordByEmail("users", "seller@burza.cz")
	cashier, _ := testApp.FindAuthRecordByEmail("users", "cashier@burza.cz")

	// Create test books
	createTestBook(t, testApp, "bookavailable01", seller.Id, event.Id, 100.0, "available")
	createTestBook(t, testApp, "booksold0000001", seller.Id, event.Id, 250.0, "bought")

	cashierToken, _ := cashier.NewAuthToken()

	// 1. Test POST /api/cashier/return-books
	var scenarioReturnBooks tests.ApiScenario
	scenarioReturnBooks = tests.ApiScenario{
		Name:            "Cashier returns books",
		Method:          http.MethodPost,
		URL:             "/api/cashier/return-books",
		Body:            strings.NewReader(`{"bookIds": ["bookavailable01"]}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"success":true`, `"count":1`, "bookavailable01"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
			sel, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
			csh, _ := app.FindAuthRecordByEmail("users", "cashier@burza.cz")
			ev, _ := app.FindFirstRecordByData("events", "active", true)
			createTestBook(tb.(*testing.T), app, "bookavailable01", sel.Id, ev.Id, 100.0, "available")
			tok, _ := csh.NewAuthToken()
			scenarioReturnBooks.Headers = map[string]string{"Authorization": tok}
		},
	}
	scenarioReturnBooks.Test(t)

	// 2. Test POST /api/cashier/returns/pay-cash
	var scenarioPayCash tests.ApiScenario
	scenarioPayCash = tests.ApiScenario{
		Name:            "Cashier pays cash to seller",
		Method:          http.MethodPost,
		URL:             "/api/cashier/returns/pay-cash",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"success":true`, `"amount":250`, `"totalUnpaid":0`},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
			sel, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
			csh, _ := app.FindAuthRecordByEmail("users", "cashier@burza.cz")
			ev, _ := app.FindFirstRecordByData("events", "active", true)
			createTestBook(tb.(*testing.T), app, "booksold0000001", sel.Id, ev.Id, 250.0, "bought")
			tok, _ := csh.NewAuthToken()
			scenarioPayCash.Headers = map[string]string{"Authorization": tok}
			scenarioPayCash.Body = strings.NewReader(`{"sellerId": "` + sel.Id + `"}`)
		},
	}
	scenarioPayCash.Test(t)

	// 3. Test GET /api/cashier/returns/seller-details?id=...
	var scenarioSellerDetails tests.ApiScenario
	scenarioSellerDetails = tests.ApiScenario{
		Name:            "Cashier fetches seller details",
		Method:          http.MethodGet,
		URL:             "/api/cashier/returns/seller-details",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"seller@burza.cz", "soldBooks", "unsoldBooks"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
			sel, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
			csh, _ := app.FindAuthRecordByEmail("users", "cashier@burza.cz")
			tok, _ := csh.NewAuthToken()
			scenarioSellerDetails.Headers = map[string]string{"Authorization": tok}
			scenarioSellerDetails.URL = "/api/cashier/returns/seller-details?id=" + sel.Id
		},
	}
	scenarioSellerDetails.Test(t)

	// 4. Test POST /api/cashier/returns/pay-cash with negative amount (rejected with 400)
	var scenarioPayCashNegative tests.ApiScenario
	scenarioPayCashNegative = tests.ApiScenario{
		Name:            "Cashier pays negative cash rejected",
		Method:          http.MethodPost,
		URL:             "/api/cashier/returns/pay-cash",
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"Částka k vyplacení nesmí být záporná"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
			sel, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
			csh, _ := app.FindAuthRecordByEmail("users", "cashier@burza.cz")
			tok, _ := csh.NewAuthToken()
			scenarioPayCashNegative.Headers = map[string]string{"Authorization": tok}
			scenarioPayCashNegative.Body = strings.NewReader(`{"sellerId": "` + sel.Id + `", "amount": -100}`)
		},
	}
	scenarioPayCashNegative.Test(t)

	// 5. Test POST /api/cashier/returns/pay-cash with excessive amount (rejected with 400)
	var scenarioPayCashExcessive tests.ApiScenario
	scenarioPayCashExcessive = tests.ApiScenario{
		Name:            "Cashier pays excessive cash rejected",
		Method:          http.MethodPost,
		URL:             "/api/cashier/returns/pay-cash",
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"přesahuje zbývající nevyplacený zůstatek"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
			sel, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
			csh, _ := app.FindAuthRecordByEmail("users", "cashier@burza.cz")
			ev, _ := app.FindFirstRecordByData("events", "active", true)
			createTestBook(tb.(*testing.T), app, "booksold0000002", sel.Id, ev.Id, 100.0, "bought")
			tok, _ := csh.NewAuthToken()
			scenarioPayCashExcessive.Headers = map[string]string{"Authorization": tok}
			scenarioPayCashExcessive.Body = strings.NewReader(`{"sellerId": "` + sel.Id + `", "amount": 9999}`)
		},
	}
	scenarioPayCashExcessive.Test(t)

	// 6. Test POST /api/cashier/returns/pay-cash by non-cashier (rejected with 403)
	var scenarioPayCashNonCashier tests.ApiScenario
	scenarioPayCashNonCashier = tests.ApiScenario{
		Name:            "Non-cashier paying cash rejected",
		Method:          http.MethodPost,
		URL:             "/api/cashier/returns/pay-cash",
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{"Pouze pokladní má přístup k této funkci"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
			sel, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
			by, _ := app.FindAuthRecordByEmail("users", "buyer@burza.cz")
			tok, _ := by.NewAuthToken()
			scenarioPayCashNonCashier.Headers = map[string]string{"Authorization": tok}
			scenarioPayCashNonCashier.Body = strings.NewReader(`{"sellerId": "` + sel.Id + `"}`)
		},
	}
	scenarioPayCashNonCashier.Test(t)

	_ = cashierToken
}
