package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestEmailTemplateSchemaAndSeeding(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()

	if err := ensureSchema(testApp); err != nil {
		t.Fatalf("ensureSchema failed: %v", err)
	}
	if err := seedInitialData(testApp); err != nil {
		t.Fatalf("seedInitialData failed: %v", err)
	}

	coll, err := testApp.FindCollectionByNameOrId("email_templates")
	if err != nil || coll == nil {
		t.Fatalf("email_templates collection should exist: %v", err)
	}

	intakeTmpl, err := testApp.FindFirstRecordByData("email_templates", "key", "intake_recap")
	if err != nil || intakeTmpl == nil {
		t.Fatalf("intake_recap template should be seeded: %v", err)
	}
	if !strings.Contains(intakeTmpl.GetString("subject"), "Rekapitulace") {
		t.Errorf("Unexpected intake_recap subject: %s", intakeTmpl.GetString("subject"))
	}

	saleTmpl, err := testApp.FindFirstRecordByData("email_templates", "key", "sale_summary")
	if err != nil || saleTmpl == nil {
		t.Fatalf("sale_summary template should be seeded: %v", err)
	}
	if !strings.Contains(saleTmpl.GetString("subject"), "Výsledky") {
		t.Errorf("Unexpected sale_summary subject: %s", saleTmpl.GetString("subject"))
	}
}

func TestRenderIntakeRecapEmail_PayoutBankVsCash(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()
	_ = ensureSchema(testApp)

	usersColl, _ := testApp.FindCollectionByNameOrId("users")
	booksColl, _ := testApp.FindCollectionByNameOrId("books")
	eventsColl, _ := testApp.FindCollectionByNameOrId("events")

	event := core.NewRecord(eventsColl)
	event.Set("name", "Testovací Burza 2026")
	event.Set("active", true)

	b1 := core.NewRecord(booksColl)
	b1.Id = "book12345678901"
	b1.Set("price", 150)
	b1.Set("accepted", true)

	b2 := core.NewRecord(booksColl)
	b2.Id = "book12345678902"
	b2.Set("price", 200)
	b2.Set("accepted", false)

	// Case 1: User chose bank payout
	bankUser := core.NewRecord(usersColl)
	bankUser.SetEmail("bankuser@gchd.cz")
	bankUser.Set("name", "Petr Bankovní")
	bankUser.Set("payoutToBank", true)
	bankUser.Set("iban", "CZ6520100000002101234567")

	subject, htmlBody, err := RenderIntakeRecapEmail(testApp, bankUser, event, []*core.Record{b1}, []*core.Record{b2}, nil)
	if err != nil {
		t.Fatalf("RenderIntakeRecapEmail failed: %v", err)
	}

	if !strings.Contains(subject, "Testovací Burza 2026") {
		t.Errorf("Subject missing event name: %s", subject)
	}
	// Must contain IBAN
	if !strings.Contains(htmlBody, "CZ6520100000002101234567") {
		t.Errorf("Expected IBAN in body, got: %s", htmlBody)
	}
	// Must NOT contain cash note
	if strings.Contains(htmlBody, "v hotovosti") {
		t.Errorf("Bank user email should NOT mention 'v hotovosti'")
	}
	// Must NOT contain internal book codes
	if strings.Contains(htmlBody, "book12345678901") || strings.Contains(htmlBody, "book12345678902") {
		t.Errorf("Email body must NOT expose internal book IDs/codes")
	}
	// Must show unaccepted warning
	if !strings.Contains(htmlBody, "Nepřijaté učebnice") {
		t.Errorf("Expected unaccepted books warning section")
	}

	// Case 2: User chose cash payout
	cashUser := core.NewRecord(usersColl)
	cashUser.SetEmail("cashuser@gchd.cz")
	cashUser.Set("name", "Jana Hotovostní")
	cashUser.Set("payoutToBank", false)

	_, cashHTML, err := RenderIntakeRecapEmail(testApp, cashUser, event, []*core.Record{b1}, nil, nil)
	if err != nil {
		t.Fatalf("RenderIntakeRecapEmail failed: %v", err)
	}
	// Must contain cash note
	if !strings.Contains(cashHTML, "v hotovosti") {
		t.Errorf("Expected cash payout note in cash user email")
	}
	// Must NOT contain bank note or IBAN
	if strings.Contains(cashHTML, "bankovní účet") {
		t.Errorf("Cash user email should NOT mention bank account")
	}
	// Must NOT show unaccepted warning because there are 0 unaccepted books
	if strings.Contains(cashHTML, "Nepřijaté učebnice") {
		t.Errorf("Email without unaccepted books should NOT show unaccepted books warning")
	}
}

func TestRenderSaleSummaryEmail_PayoutAndStatus(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()
	_ = ensureSchema(testApp)

	usersColl, _ := testApp.FindCollectionByNameOrId("users")
	booksColl, _ := testApp.FindCollectionByNameOrId("books")
	eventsColl, _ := testApp.FindCollectionByNameOrId("events")

	event := core.NewRecord(eventsColl)
	event.Set("name", "Podzimní burza")
	event.Set("active", true)

	b1 := core.NewRecord(booksColl)
	b1.Id = "soldbook1234567"
	b1.Set("price", 150)
	b1.Set("accepted", true)
	b1.Set("status", "bought")

	b2 := core.NewRecord(booksColl)
	b2.Id = "unsoldbook12345"
	b2.Set("price", 120)
	b2.Set("accepted", true)
	b2.Set("status", "available")

	user := core.NewRecord(usersColl)
	user.SetEmail("prodejce@gchd.cz")
	user.Set("name", "Eva Prodejní")
	user.Set("payoutToBank", true)
	user.Set("iban", "CZ00000000001234567890")

	subject, htmlBody, err := RenderSaleSummaryEmail(testApp, user, event, []*core.Record{b1, b2}, nil)
	if err != nil {
		t.Fatalf("RenderSaleSummaryEmail failed: %v", err)
	}

	if !strings.Contains(subject, "Podzimní burza") {
		t.Errorf("Subject missing event name: %s", subject)
	}
	// Must show total payout of 150 Kč (only the sold book!)
	if !strings.Contains(htmlBody, "150 Kč") {
		t.Errorf("Expected 150 Kč total payout in email")
	}
	// Must show sold count
	if !strings.Contains(htmlBody, "1 z 2 ks") {
		t.Errorf("Expected '1 z 2 ks' sold in email")
	}
	// Must show PRODÁNO and NEPRODÁNO badges
	if !strings.Contains(htmlBody, "PRODÁNO") {
		t.Errorf("Expected PRODÁNO badge")
	}
	if !strings.Contains(htmlBody, "NEPRODÁNO") {
		t.Errorf("Expected NEPRODÁNO badge")
	}
	// Must NOT contain internal book codes
	if strings.Contains(htmlBody, "soldbook1234567") || strings.Contains(htmlBody, "unsoldbook12345") {
		t.Errorf("Email body must NOT expose internal book codes")
	}
}

func TestEmailEndpointsRequireCashierAuth(t *testing.T) {
	// Request without cashier auth should be rejected
	scenario := tests.ApiScenario{
		Name:            "Stats endpoint rejects unauthenticated request",
		Method:          http.MethodGet,
		URL:             "/api/admin/emails/stats",
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{"The request requires valid record authorization token"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)
		},
	}
	scenario.Test(t)

	// Non-cashier user should be forbidden / unauthorized
	var scenarioCashierCheck tests.ApiScenario
	scenarioCashierCheck = tests.ApiScenario{
		Name:            "Stats endpoint rejects non-cashier user",
		Method:          http.MethodGet,
		URL:             "/api/admin/emails/stats",
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{"Pouze pokladník má přístup"},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)

			usersColl, _ := app.FindCollectionByNameOrId("users")
			normalUser := core.NewRecord(usersColl)
			normalUser.SetEmail("regular@burza.cz")
			normalUser.SetPassword("testpass123")
			normalUser.Set("isCashier", false)
			_ = app.Save(normalUser)

			token, _ := normalUser.NewAuthToken()
			if scenarioCashierCheck.Headers == nil {
				scenarioCashierCheck.Headers = make(map[string]string)
			}
			scenarioCashierCheck.Headers["Authorization"] = token
		},
	}
	scenarioCashierCheck.Test(t)

	// Cashier user should succeed
	var scenarioCashierAllowed tests.ApiScenario
	scenarioCashierAllowed = tests.ApiScenario{
		Name:            "Stats endpoint allows cashier user",
		Method:          http.MethodGet,
		URL:             "/api/admin/emails/stats",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"totalSellers":`, `"totalBooks":`},
		TestAppFactory:  testAppFactory,
		BeforeTestFunc: func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			beforeTest(tb, app, e)

			cashier, err := app.FindAuthRecordByEmail("users", "cashier@burza.cz")
			if err != nil {
				tb.Fatalf("cashier user not found: %v", err)
			}
			token, _ := cashier.NewAuthToken()
			if scenarioCashierAllowed.Headers == nil {
				scenarioCashierAllowed.Headers = make(map[string]string)
			}
			scenarioCashierAllowed.Headers["Authorization"] = token
		},
	}
	scenarioCashierAllowed.Test(t)
}
