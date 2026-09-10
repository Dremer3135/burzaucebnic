package main

import (
	"image/color"
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

func testAppFactory(t testing.TB) *tests.TestApp {
	testApp, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("Failed to create TestApp: %v", err)
	}
	registerHooks(testApp)
	return testApp
}

func beforeTest(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
	if err := ensureSchema(app); err != nil {
		t.Fatalf("ensureSchema: %v", err)
	}
	if err := seedInitialData(app); err != nil {
		t.Logf("seed notice: %v", err)
	}
	registerApiEndpoints(e)
}

func TestBookCreationRejectsUserID(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()

	if err := ensureSchema(testApp); err != nil {
		t.Fatalf("ensureSchema failed: %v", err)
	}

	// 1. Create a test user with a fixed 15-char ID
	usersColl, err := testApp.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("Users collection not found: %v", err)
	}
	targetUserId := "u1234567890abcd"
	user := core.NewRecord(usersColl)
	user.Id = targetUserId
	user.Set("email", "testuser_unique@burza.cz")
	user.Set("username", "testuseruniq")
	user.SetPassword("password123")
	user.SetVerified(true)
	if err := testApp.Save(user); err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	// 2. Test model-level save: attempting to save a book with the user ID must fail
	booksColl, err := testApp.FindCollectionByNameOrId("books")
	if err != nil {
		t.Fatalf("Books collection not found: %v", err)
	}

	eventsColl, _ := testApp.FindCollectionByNameOrId("events")
	eventRecord := core.NewRecord(eventsColl)
	eventRecord.Set("name", "Test Event")
	eventRecord.Set("active", true)
	_ = testApp.Save(eventRecord)

	coverBytes, fileName, err := generateDummyCoverBytes("Test Title", "Author", color.RGBA{R: 100, G: 100, B: 100, A: 255})
	if err != nil {
		t.Fatalf("generateDummyCoverBytes failed: %v", err)
	}
	dummyFile, err := filesystem.NewFileFromBytes(coverBytes, fileName)
	if err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
	}

	collidingBook := core.NewRecord(booksColl)
	collidingBook.Id = targetUserId
	collidingBook.Set("seller", user.Id)
	collidingBook.Set("event", eventRecord.Id)
	collidingBook.Set("price", 100)
	collidingBook.Set("status", "available")
	collidingBook.Set("photo", dummyFile)

	saveErr := testApp.Save(collidingBook)
	if saveErr == nil {
		t.Fatal("Expected error when saving book with an existing user ID, but got nil")
	}
	if !strings.Contains(saveErr.Error(), "kód knihy se nesmí shodovat s ID uživatele") {
		t.Fatalf("Unexpected error message: %v", saveErr)
	}

	// 3. Saving a book with a normal available ID must succeed
	validCoverBytes, validFileName, _ := generateDummyCoverBytes("Valid Book", "Author", color.RGBA{R: 50, G: 150, B: 50, A: 255})
	validDummyFile, _ := filesystem.NewFileFromBytes(validCoverBytes, validFileName)
	validBook := core.NewRecord(booksColl)
	validBook.Id = "b1234567890abcd"
	validBook.Set("seller", user.Id)
	validBook.Set("event", eventRecord.Id)
	validBook.Set("price", 150)
	validBook.Set("status", "available")
	validBook.Set("photo", validDummyFile)
	if err := testApp.Save(validBook); err != nil {
		t.Fatalf("Failed to save valid book: %v", err)
	}
}

func TestApiEndpoints(t *testing.T) {
	sampleBookId := "matgymn00000001"

	scenarios := []struct {
		scenario tests.ApiScenario
		urlFunc  func(app *tests.TestApp) string
	}{
		{
			scenario: tests.ApiScenario{
				Name:           "GET /api/book-price with user ID returns 400 user_id (not 404)",
				Method:         http.MethodGet,
				TestAppFactory: testAppFactory,
				ExpectedStatus: http.StatusBadRequest,
				ExpectedContent: []string{
					`"user_id"`,
					`"Toto je kód uživatele, nikoliv učebnice."`,
				},
			},
			urlFunc: func(app *tests.TestApp) string {
				seller, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
				return "/api/book-price?id=" + seller.Id
			},
		},
		{
			scenario: tests.ApiScenario{
				Name:           "GET /api/book-price with existing book returns 200",
				Method:         http.MethodGet,
				URL:            "/api/book-price?id=" + sampleBookId,
				TestAppFactory: testAppFactory,
				ExpectedStatus: http.StatusOK,
				ExpectedContent: []string{
					`"` + sampleBookId + `"`,
					`"available"`,
					`150`,
				},
			},
		},
		{
			scenario: tests.ApiScenario{
				Name:           "GET /api/book-price with nonexistent ID returns 404",
				Method:         http.MethodGet,
				URL:            "/api/book-price?id=c1234567890abcd",
				TestAppFactory: testAppFactory,
				ExpectedStatus: http.StatusNotFound,
				ExpectedContent: []string{
					`"Kniha nebyla nalezena."`,
				},
			},
		},
		{
			scenario: tests.ApiScenario{
				Name:           "GET /api/check-book-code with user ID returns status user",
				Method:         http.MethodGet,
				TestAppFactory: testAppFactory,
				ExpectedStatus: http.StatusOK,
				ExpectedContent: []string{
					`"user"`,
					`"Toto je kód uživatele, nikoliv učebnice."`,
				},
			},
			urlFunc: func(app *tests.TestApp) string {
				seller, _ := app.FindAuthRecordByEmail("users", "seller@burza.cz")
				return "/api/check-book-code?code=" + seller.Id
			},
		},
		{
			scenario: tests.ApiScenario{
				Name:           "GET /api/check-book-code with existing book returns status used",
				Method:         http.MethodGet,
				URL:            "/api/check-book-code?code=" + sampleBookId,
				TestAppFactory: testAppFactory,
				ExpectedStatus: http.StatusOK,
				ExpectedContent: []string{
					`"used"`,
				},
			},
		},
		{
			scenario: tests.ApiScenario{
				Name:           "GET /api/check-book-code with free code returns status available",
				Method:         http.MethodGet,
				URL:            "/api/check-book-code?code=f1234567890abcd",
				TestAppFactory: testAppFactory,
				ExpectedStatus: http.StatusOK,
				ExpectedContent: []string{
					`"available"`,
					`"Kód je volný."`,
				},
			},
		},
	}

	for i := range scenarios {
		tc := &scenarios[i]
		t.Run(tc.scenario.Name, func(t *testing.T) {
			tc.scenario.BeforeTestFunc = func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				beforeTest(tb, app, e)

				buyer, err := app.FindAuthRecordByEmail("users", "buyer@burza.cz")
				if err != nil {
					tb.Fatalf("Failed to find buyer: %v", err)
				}
				token, err := buyer.NewAuthToken()
				if err != nil {
					tb.Fatalf("Failed to generate buyer token: %v", err)
				}

				if tc.scenario.Headers == nil {
					tc.scenario.Headers = make(map[string]string)
				}
				tc.scenario.Headers["Authorization"] = token

				if tc.urlFunc != nil {
					tc.scenario.URL = tc.urlFunc(app)
				}
			}

			tc.scenario.Test(t)
		})
	}
}

func TestEventsCollectionHasMaintenanceBreak(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()

	if err := ensureSchema(testApp); err != nil {
		t.Fatalf("ensureSchema failed: %v", err)
	}

	eventsColl, err := testApp.FindCollectionByNameOrId("events")
	if err != nil {
		t.Fatalf("events collection not found: %v", err)
	}

	field := eventsColl.Fields.GetByName("maintenance_break")
	if field == nil {
		t.Fatalf("expected maintenance_break field on events collection, got nil")
	}

	if field.Type() != core.FieldTypeBool {
		t.Fatalf("expected maintenance_break to be bool field, got %s", field.Type())
	}
}

func TestCashierSearchBooks(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()

	scenarios := []struct {
		name           string
		tokenUserEmail string
		query          string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "search-books rejected without auth",
			tokenUserEmail: "",
			query:          "mat",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "The request requires valid record authorization token.",
		},
		{
			name:           "search-books rejected for regular non-cashier user",
			tokenUserEmail: "seller@burza.cz",
			query:          "mat",
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Pouze pokladní má přístup",
		},
		{
			name:           "search-books allowed for cashier and finds book",
			tokenUserEmail: "cashier@burza.cz",
			query:          "matgymn",
			expectedStatus: http.StatusOK,
			expectedBody:   "matgymn00000001",
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			scenario := tests.ApiScenario{
				Name:           s.name,
				Method:         http.MethodGet,
				URL:            "/api/cashier/search-books?query=" + s.query,
				TestAppFactory: testAppFactory,
				ExpectedStatus: s.expectedStatus,
				ExpectedContent: []string{
					s.expectedBody,
				},
			}
			scenario.BeforeTestFunc = func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				beforeTest(tb, app, e)
				if s.tokenUserEmail != "" {
					u, err := app.FindAuthRecordByEmail("users", s.tokenUserEmail)
					if err != nil {
						tb.Fatalf("Failed to find user %s: %v", s.tokenUserEmail, err)
					}
					token, err := u.NewAuthToken()
					if err != nil {
						tb.Fatalf("Failed to generate token: %v", err)
					}
					scenario.Headers = map[string]string{
						"Authorization": token,
					}
				}
			}
			scenario.Test(t)
		})
	}
}

func TestCashierCreateUser(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()

	scenarios := []struct {
		name           string
		tokenUserEmail string
		body           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "create-user rejected without auth",
			tokenUserEmail: "",
			body:           `{"email":"newstudent@burza.cz","password":"password123","passwordConfirm":"password123","name":"Student Novy"}`,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "The request requires valid record authorization token.",
		},
		{
			name:           "create-user rejected for non-cashier",
			tokenUserEmail: "seller@burza.cz",
			body:           `{"email":"newstudent@burza.cz","password":"password123","passwordConfirm":"password123","name":"Student Novy"}`,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Pouze pokladní má přístup",
		},
		{
			name:           "create-user allowed for cashier",
			tokenUserEmail: "cashier@burza.cz",
			body:           `{"email":"newstudent@burza.cz","password":"password123","passwordConfirm":"password123","name":"Student Novy"}`,
			expectedStatus: http.StatusOK,
			expectedBody:   `"email":"newstudent@burza.cz"`,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			scenario := tests.ApiScenario{
				Name:           s.name,
				Method:         http.MethodPost,
				URL:            "/api/cashier/create-user",
				Body:           strings.NewReader(s.body),
				TestAppFactory: testAppFactory,
				ExpectedStatus: s.expectedStatus,
				ExpectedContent: []string{
					s.expectedBody,
				},
			}
			scenario.BeforeTestFunc = func(tb testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				beforeTest(tb, app, e)
				if s.tokenUserEmail != "" {
					u, err := app.FindAuthRecordByEmail("users", s.tokenUserEmail)
					if err != nil {
						tb.Fatalf("Failed to find user %s: %v", s.tokenUserEmail, err)
					}
					token, err := u.NewAuthToken()
					if err != nil {
						tb.Fatalf("Failed to generate token: %v", err)
					}
					scenario.Headers = map[string]string{
						"Authorization": token,
					}
				}
			}
			scenario.Test(t)
		})
	}
}

func TestCashierSellAsSomeoneElseBookRules(t *testing.T) {
	testApp := testAppFactory(t)
	defer testApp.Cleanup()

	if err := ensureSchema(testApp); err != nil {
		t.Fatalf("ensureSchema failed: %v", err)
	}
	if err := seedInitialData(testApp); err != nil {
		t.Logf("seed notice: %v", err)
	}

	booksColl, err := testApp.FindCollectionByNameOrId("books")
	if err != nil {
		t.Fatalf("Books collection not found: %v", err)
	}

	if booksColl.CreateRule == nil || !strings.Contains(*booksColl.CreateRule, "@request.auth.isCashier = true") {
		t.Fatalf("expected books.CreateRule to allow isCashier, got %v", booksColl.CreateRule)
	}
	if booksColl.DeleteRule == nil || !strings.Contains(*booksColl.DeleteRule, "@request.auth.isCashier = true") {
		t.Fatalf("expected books.DeleteRule to allow isCashier, got %v", booksColl.DeleteRule)
	}
}


