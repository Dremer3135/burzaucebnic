package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestParseEnvReader(t *testing.T) {
	content := `
# This is a comment
TEST_FIO_VAR1=hello_world
TEST_FIO_VAR2="quoted_value"
TEST_FIO_VAR3='single_quoted'
INVALID_LINE_WITHOUT_EQUALS
EMPTY_VAR=
`
	parseEnvReader(strings.NewReader(content))

	if os.Getenv("TEST_FIO_VAR1") != "hello_world" {
		t.Errorf("Expected TEST_FIO_VAR1=hello_world, got %s", os.Getenv("TEST_FIO_VAR1"))
	}
	if os.Getenv("TEST_FIO_VAR2") != "quoted_value" {
		t.Errorf("Expected TEST_FIO_VAR2=quoted_value, got %s", os.Getenv("TEST_FIO_VAR2"))
	}
	if os.Getenv("TEST_FIO_VAR3") != "single_quoted" {
		t.Errorf("Expected TEST_FIO_VAR3=single_quoted, got %s", os.Getenv("TEST_FIO_VAR3"))
	}
}

func TestParseFioResponse(t *testing.T) {
	rawJSON := `{
		"accountStatement": {
			"info": {
				"accountId": "2101234567",
				"currency": "CZK"
			},
			"transactionList": {
				"transaction": [
					{
						"column0": {"value": "2026-09-08+0200", "id": 0, "name": "Datum"},
						"column1": {"value": 350.0, "id": 1, "name": "Objem"},
						"column2": {"value": "123456789", "id": 2, "name": "Protiúčet"},
						"column3": {"value": "0800", "id": 3, "name": "Kód banky"},
						"column5": {"value": "10042", "id": 5, "name": "VS"},
						"column10": {"value": "Jan Novák", "id": 10, "name": "Název protiúčtu"},
						"column14": {"value": "CZK", "id": 14, "name": "Měna"},
						"column16": {"value": "Platba za knihy", "id": 16, "name": "Zpráva"},
						"column22": {"value": 2512345678, "id": 22, "name": "ID pohybu"}
					},
					{
						"column0": {"value": "2026-09-08+0200", "id": 0, "name": "Datum"},
						"column1": {"value": -50.0, "id": 1, "name": "Objem"},
						"column5": {"value": 10043, "id": 5, "name": "VS as number"},
						"column22": {"value": "2512345679", "id": 22, "name": "ID pohybu as string"}
					}
				]
			}
		}
	}`

	txs, err := parseFioResponse([]byte(rawJSON))
	if err != nil {
		t.Fatalf("parseFioResponse failed: %v", err)
	}

	if len(txs) != 2 {
		t.Fatalf("Expected 2 transactions, got %d", len(txs))
	}

	tx1 := txs[0]
	if tx1.ID != "2512345678" {
		t.Errorf("Expected ID '2512345678', got '%s'", tx1.ID)
	}
	if tx1.Amount != 350.0 {
		t.Errorf("Expected Amount 350.0, got %f", tx1.Amount)
	}
	if tx1.VS != "10042" {
		t.Errorf("Expected VS '10042', got '%s'", tx1.VS)
	}
	if tx1.SenderName != "Jan Novák" {
		t.Errorf("Expected SenderName 'Jan Novák', got '%s'", tx1.SenderName)
	}

	tx2 := txs[1]
	if tx2.Amount != -50.0 {
		t.Errorf("Expected Amount -50.0, got %f", tx2.Amount)
	}
	if tx2.VS != "10043" {
		t.Errorf("Expected VS '10043' from numeric column, got '%s'", tx2.VS)
	}
	if tx2.ID != "2512345679" {
		t.Errorf("Expected ID '2512345679', got '%s'", tx2.ID)
	}
}

func TestFioRateLimiter(t *testing.T) {
	resetFioRateLimit()

	ok, remaining := checkFioRateLimit()
	if !ok || remaining != 0 {
		t.Errorf("Expected rate limit to allow request initially, got ok=%v, remaining=%d", ok, remaining)
	}

	recordFioRequest()

	ok, remaining = checkFioRateLimit()
	if ok || remaining <= 0 || remaining > 30 {
		t.Errorf("Expected rate limit to reject request after recordFioRequest, got ok=%v, remaining=%d", ok, remaining)
	}

	resetFioRateLimit()
	ok, _ = checkFioRateLimit()
	if !ok {
		t.Errorf("Expected rate limit to allow request after reset")
	}
}

func TestSyncFioPaymentsMatching(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()
	_ = ensureSchema(testApp)
	_ = seedInitialData(testApp)

	// Mock Fio server
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
							"column0":  map[string]any{"value": "2026-09-08+0200"},
							"column1":  map[string]any{"value": 150.0},
							"column5":  map[string]any{"value": "10050"},
							"column10": map[string]any{"value": "Petr Kupující"},
							"column14": map[string]any{"value": "CZK"},
							"column22": map[string]any{"value": "tx_mock_1"},
						},
						{
							"column0":  map[string]any{"value": "2026-09-08+0200"},
							"column1":  map[string]any{"value": 50.0}, // Underpayment for payment 2 (needs 120)
							"column5":  map[string]any{"value": "10051"},
							"column10": map[string]any{"value": "Šetřílek"},
							"column14": map[string]any{"value": "CZK"},
							"column22": map[string]any{"value": "tx_mock_2"},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockFio.Close()

	// Temporarily override fioBaseURL to mock server
	origURL := fioBaseURL
	fioBaseURL = mockFio.URL
	defer func() { fioBaseURL = origURL }()

	resetFioRateLimit()

	buyer, err := testApp.FindAuthRecordByEmail("users", "buyer@burza.cz")
	if err != nil || buyer == nil {
		t.Fatalf("Buyer not found: %v", err)
	}

	paymentsColl, _ := testApp.FindCollectionByNameOrId("payments")

	// Payment 1: VS 10050, 150 Kč for matgymn00000001 (will match tx_mock_1)
	p1 := core.NewRecord(paymentsColl)
	p1.Set("variableSymbol", 10050)
	p1.Set("buyer", buyer.Id)
	p1.Set("books", []string{"matgymn00000001"})
	p1.Set("totalAmount", 150)
	p1.Set("method", "qr")
	p1.Set("status", "pending")
	if err := testApp.Save(p1); err != nil {
		t.Fatalf("Failed to save p1: %v", err)
	}

	// Payment 2: VS 10051, 120 Kč for fyzsbirka000002 (will NOT match due to underpayment of 50 Kč)
	p2 := core.NewRecord(paymentsColl)
	p2.Set("variableSymbol", 10051)
	p2.Set("buyer", buyer.Id)
	p2.Set("books", []string{"fyzsbirka000002"})
	p2.Set("totalAmount", 120)
	p2.Set("method", "qr")
	p2.Set("status", "pending")
	if err := testApp.Save(p2); err != nil {
		t.Fatalf("Failed to save p2: %v", err)
	}

	result, err := SyncFioPayments(testApp, "test_token_123")
	if err != nil {
		t.Fatalf("SyncFioPayments failed: %v", err)
	}

	if result.MatchedCount != 1 {
		t.Fatalf("Expected 1 matched payment, got %d", result.MatchedCount)
	}
	if result.MatchedPayments[0].PaymentId != p1.Id {
		t.Errorf("Expected matched payment ID %s, got %s", p1.Id, result.MatchedPayments[0].PaymentId)
	}

	// Verify Payment 1 in DB
	refreshedP1, _ := testApp.FindRecordById("payments", p1.Id)
	if refreshedP1.GetString("status") != "completed" {
		t.Errorf("Expected p1 status completed, got %s", refreshedP1.GetString("status"))
	}
	if refreshedP1.GetString("confirmation_type") != "automatic" {
		t.Errorf("Expected p1 confirmation_type automatic, got %s", refreshedP1.GetString("confirmation_type"))
	}
	if refreshedP1.GetString("fio_transaction_id") != "tx_mock_1" {
		t.Errorf("Expected p1 fio_transaction_id tx_mock_1, got %s", refreshedP1.GetString("fio_transaction_id"))
	}

	// Verify Book 1 is now bought
	refreshedBook1, _ := testApp.FindRecordById("books", "matgymn00000001")
	if refreshedBook1.GetString("status") != "bought" {
		t.Errorf("Expected book1 status bought, got %s", refreshedBook1.GetString("status"))
	}

	// Verify Payment 2 in DB stayed pending
	refreshedP2, _ := testApp.FindRecordById("payments", p2.Id)
	if refreshedP2.GetString("status") != "pending" {
		t.Errorf("Expected p2 status pending, got %s", refreshedP2.GetString("status"))
	}
	if refreshedP2.GetString("confirmation_type") != "" {
		t.Errorf("Expected p2 confirmation_type empty, got %s", refreshedP2.GetString("confirmation_type"))
	}
}

func TestSyncFioPaymentsEndpointRequiresCashier(t *testing.T) {
	// 1. Unauthenticated request should return 401
	scenarioUnauth := tests.ApiScenario{
		Name:            "Fio sync endpoint rejects unauthenticated",
		Method:          http.MethodPost,
		URL:             "/api/cashier/sync-fio-payments",
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{"The request requires valid record authorization token"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
		},
	}
	scenarioUnauth.Test(t)

	// 2. Non-cashier request should return 403
	var scenarioNonCashier tests.ApiScenario
	scenarioNonCashier = tests.ApiScenario{
		Name:            "Fio sync endpoint rejects regular user",
		Method:          http.MethodPost,
		URL:             "/api/cashier/sync-fio-payments",
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{"Pouze pokladník má oprávnění"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)

			usersColl, _ := app.FindCollectionByNameOrId("users")
			user := core.NewRecord(usersColl)
			user.SetEmail("regularbuyer@burza.cz")
			user.SetPassword("testpass123")
			user.Set("isCashier", false)
			_ = app.Save(user)

			token, _ := user.NewAuthToken()
			if scenarioNonCashier.Headers == nil {
				scenarioNonCashier.Headers = make(map[string]string)
			}
			scenarioNonCashier.Headers["Authorization"] = token
		},
	}
	scenarioNonCashier.Test(t)
}
