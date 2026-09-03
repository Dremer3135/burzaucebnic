package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tools/types"
)

var vsLock sync.Mutex

func main() {
	app := pocketbase.New()

	// 1. Hook: Duplicate Data Matrix code validation before creating a book
	app.OnRecordBeforeCreateRequest("books").Add(func(e *core.RecordCreateEvent) error {
		code := strings.TrimSpace(e.Record.GetString("code"))
		if code == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Kód Data Matrix je povinný.")
		}
		existing, _ := app.Dao().FindFirstRecordByData("books", "code", code)
		if existing != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Kód '%s' je již zaregistrován.", code))
		}
		return nil
	})

	// 2. Hook: FFmpeg compression of book photo after upload
	app.OnRecordAfterCreateRequest("books").Add(func(e *core.RecordCreateEvent) error {
		photoName := e.Record.GetString("photo")
		if photoName == "" {
			return nil
		}
		filePath := filepath.Join(app.DataDir(), "storage", e.Record.BaseFilesPath(), photoName)
		go compressImageWithFFmpeg(filePath)
		return nil
	})

	// 3. Schema & Seed setup on startup, custom API routes & reaper
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		if err := ensureSchema(app); err != nil {
			log.Printf("[SCHEMA ERROR] %v", err)
			return err
		}

		if err := seedInitialData(app); err != nil {
			log.Printf("[SEED ERROR] %v", err)
		}

		registerApiEndpoints(app, e)
		startCheckoutReaper(app)

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// compressImageWithFFmpeg scales image down to 720p width maintaining aspect ratio and compresses JPEG
func compressImageWithFFmpeg(filePath string) {
	// Wait a moment for file to be flushed to disk
	time.Sleep(100 * time.Millisecond)

	if _, err := os.Stat(filePath); err != nil {
		return
	}

	tmpOutput := filePath + ".opt.jpg"
	// Scale to max 720 width (maintain aspect ratio) and use jpeg compression
	cmd := exec.Command("ffmpeg", "-y", "-i", filePath, "-vf", "scale='min(720,iw)':-2", "-q:v", "3", tmpOutput)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[FFMPEG COMPRESSION FAILED] %v: %s", err, string(out))
		return
	}

	// Replace original with optimized version
	if err := os.Rename(tmpOutput, filePath); err != nil {
		log.Printf("[FFMPEG RENAME FAILED] %v", err)
	} else {
		log.Printf("[FFMPEG] Successfully optimized %s to 720p", filePath)
	}
}

// ensureSchema initializes collections if they do not exist
func ensureSchema(app *pocketbase.PocketBase) error {
	dao := app.Dao()

	// 1. Events collection
	eventsColl, _ := dao.FindCollectionByNameOrId("events")
	if eventsColl == nil {
		eventsColl = &models.Collection{
			Name:       "events",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""), // Public
			ViewRule:   types.Pointer(""), // Public
			CreateRule: types.Pointer("@request.auth.isCashier = true"),
			UpdateRule: types.Pointer("@request.auth.isCashier = true"),
			DeleteRule: types.Pointer("@request.auth.isCashier = true"),
			Schema: schema.NewSchema(
				&schema.SchemaField{Name: "name", Type: schema.FieldTypeText, Required: true},
				&schema.SchemaField{Name: "active", Type: schema.FieldTypeBool},
				&schema.SchemaField{Name: "sellStart", Type: schema.FieldTypeDate},
				&schema.SchemaField{Name: "sellEnd", Type: schema.FieldTypeDate},
				&schema.SchemaField{Name: "buyStart", Type: schema.FieldTypeDate},
				&schema.SchemaField{Name: "buyEnd", Type: schema.FieldTypeDate},
				&schema.SchemaField{Name: "bankAccount", Type: schema.FieldTypeText},
				&schema.SchemaField{Name: "iban", Type: schema.FieldTypeText},
				&schema.SchemaField{Name: "currency", Type: schema.FieldTypeText},
			),
		}
		if err := dao.SaveCollection(eventsColl); err != nil {
			return fmt.Errorf("creating events collection: %w", err)
		}
	}

	// 2. Books collection
	booksColl, _ := dao.FindCollectionByNameOrId("books")
	if booksColl == nil {
		maxSelectOne := 1
		booksColl = &models.Collection{
			Name:       "books",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer("@request.auth.id != ''"),
			ViewRule:   types.Pointer("@request.auth.id != ''"),
			CreateRule: types.Pointer("@request.auth.id != '' && @request.auth.id = @request.data.seller"),
			UpdateRule: types.Pointer("@request.auth.id != '' && (@request.auth.id = seller || @request.auth.isCashier = true)"),
			DeleteRule: types.Pointer("@request.auth.id != '' && @request.auth.id = seller && status = 'available'"),
			Schema: schema.NewSchema(
				&schema.SchemaField{Name: "code", Type: schema.FieldTypeText, Required: true},
				&schema.SchemaField{
					Name: "seller", Type: schema.FieldTypeRelation, Required: true,
					Options: &schema.RelationOptions{CollectionId: "users", MaxSelect: &maxSelectOne},
				},
				&schema.SchemaField{
					Name: "buyer", Type: schema.FieldTypeRelation,
					Options: &schema.RelationOptions{CollectionId: "users", MaxSelect: &maxSelectOne},
				},
				&schema.SchemaField{
					Name: "event", Type: schema.FieldTypeRelation, Required: true,
					Options: &schema.RelationOptions{CollectionId: eventsColl.Id, MaxSelect: &maxSelectOne},
				},
				&schema.SchemaField{Name: "price", Type: schema.FieldTypeNumber, Required: true},
				&schema.SchemaField{
					Name: "photo", Type: schema.FieldTypeFile, Required: true,
					Options: &schema.FileOptions{
						MaxSelect: 1,
						MaxSize:   10485760,
						MimeTypes: []string{"image/jpeg", "image/png", "image/webp"},
						Thumbs:    []string{"100x150"},
					},
				},
				&schema.SchemaField{
					Name: "status", Type: schema.FieldTypeSelect,
					Options: &schema.SelectOptions{
						Values:    []string{"available", "checkout", "bought"},
						MaxSelect: 1,
					},
				},
				&schema.SchemaField{Name: "checkoutExpiresAt", Type: schema.FieldTypeDate},
			),
			Indexes: types.JsonArray[string]{
				"CREATE UNIQUE INDEX idx_books_code ON books (code)",
			},
		}
		if err := dao.SaveCollection(booksColl); err != nil {
			return fmt.Errorf("creating books collection: %w", err)
		}
	}

	// 3. Payments collection
	paymentsColl, _ := dao.FindCollectionByNameOrId("payments")
	if paymentsColl == nil {
		maxSelectOne := 1
		paymentsColl = &models.Collection{
			Name:       "payments",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer("@request.auth.isCashier = true || @request.auth.id = buyer"),
			ViewRule:   types.Pointer("@request.auth.isCashier = true || @request.auth.id = buyer"),
			CreateRule: types.Pointer("@request.auth.isCashier = true"),
			UpdateRule: types.Pointer("@request.auth.isCashier = true"),
			DeleteRule: types.Pointer("@request.auth.isCashier = true"),
			Schema: schema.NewSchema(
				&schema.SchemaField{Name: "variableSymbol", Type: schema.FieldTypeNumber, Required: true},
				&schema.SchemaField{
					Name: "buyer", Type: schema.FieldTypeRelation, Required: true,
					Options: &schema.RelationOptions{CollectionId: "users", MaxSelect: &maxSelectOne},
				},
				&schema.SchemaField{
					Name: "books", Type: schema.FieldTypeRelation, Required: true,
					Options: &schema.RelationOptions{CollectionId: booksColl.Id},
				},
				&schema.SchemaField{Name: "totalAmount", Type: schema.FieldTypeNumber, Required: true},
				&schema.SchemaField{
					Name: "method", Type: schema.FieldTypeSelect, Required: true,
					Options: &schema.SelectOptions{Values: []string{"qr", "cash"}, MaxSelect: 1},
				},
				&schema.SchemaField{
					Name: "status", Type: schema.FieldTypeSelect,
					Options: &schema.SelectOptions{Values: []string{"pending", "completed", "cancelled"}, MaxSelect: 1},
				},
				&schema.SchemaField{
					Name: "cashier", Type: schema.FieldTypeRelation,
					Options: &schema.RelationOptions{CollectionId: "users", MaxSelect: &maxSelectOne},
				},
			),
		}
		if err := dao.SaveCollection(paymentsColl); err != nil {
			return fmt.Errorf("creating payments collection: %w", err)
		}
	}

	// 4. Update users collection with isCashier, buy fields, and view/list rules for relations
	usersColl, err := dao.FindCollectionByNameOrId("users")
	if err == nil {
		var changed bool
		if usersColl.Schema.GetFieldByName("isCashier") == nil {
			usersColl.Schema.AddField(&schema.SchemaField{
				Name: "isCashier",
				Type: schema.FieldTypeBool,
			})
			changed = true
		}
		if usersColl.Schema.GetFieldByName("buy") == nil {
			usersColl.Schema.AddField(&schema.SchemaField{
				Name: "buy",
				Type: schema.FieldTypeRelation,
				Options: &schema.RelationOptions{
					CollectionId: booksColl.Id,
				},
			})
			changed = true
		}
		if usersColl.ViewRule == nil || *usersColl.ViewRule != "@request.auth.id != ''" {
			usersColl.ViewRule = types.Pointer("@request.auth.id != ''")
			usersColl.ListRule = types.Pointer("@request.auth.id != ''")
			changed = true
		}
		if changed {
			if err := dao.SaveCollection(usersColl); err != nil {
				return fmt.Errorf("updating users schema: %w", err)
			}
		}
	}


	return nil
}

// generateDummyCoverBytes generates a simple colored JPEG cover image
func generateDummyCoverBytes(title, author string, bg color.RGBA) ([]byte, string, error) {
	width, height := 600, 900
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill background
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	// Draw inner border
	borderColor := color.RGBA{R: 255, G: 255, B: 255, A: 160}
	for x := 30; x < width-30; x++ {
		img.Set(x, 30, borderColor)
		img.Set(x, height-30, borderColor)
	}
	for y := 30; y < height-30; y++ {
		img.Set(30, y, borderColor)
		img.Set(width-30, y, borderColor)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", err
	}

	fileName := fmt.Sprintf("sample_%s.jpg", strings.ToLower(strings.ReplaceAll(title, " ", "_")))
	return buf.Bytes(), fileName, nil
}

// seedInitialData creates test accounts, active event, and sample books
func seedInitialData(app *pocketbase.PocketBase) error {
	dao := app.Dao()

	// 1. Active Event
	event, _ := dao.FindFirstRecordByData("events", "active", true)
	if event == nil {
		eventsColl, err := dao.FindCollectionByNameOrId("events")
		if err != nil {
			return err
		}
		event = models.NewRecord(eventsColl)
		event.Set("name", "Podzimní burza učebnic 2026")
		event.Set("active", true)
		event.Set("sellStart", time.Now().Add(-24*time.Hour).UTC().Format("2006-01-02 15:04:05.000Z"))
		event.Set("sellEnd", time.Now().Add(30*24*time.Hour).UTC().Format("2006-01-02 15:04:05.000Z"))
		event.Set("buyStart", time.Now().Add(-24*time.Hour).UTC().Format("2006-01-02 15:04:05.000Z"))
		event.Set("buyEnd", time.Now().Add(30*24*time.Hour).UTC().Format("2006-01-02 15:04:05.000Z"))
		event.Set("bankAccount", "2101234567/2010")
		event.Set("iban", "CZ6520100000002101234567")
		event.Set("currency", "CZK")
		if err := dao.SaveRecord(event); err != nil {
			log.Printf("[SEED] Failed to create event: %v", err)
		} else {
			log.Printf("[SEED] Created active event: %s", event.GetString("name"))
		}
	}

	// 2. Test Users
	usersColl, err := dao.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	type testUser struct {
		email     string
		username  string
		name      string
		password  string
		isCashier bool
	}

	testUsers := []testUser{
		{email: "cashier@burza.cz", username: "cashier", name: "Pokladní Jana", password: "heslo123", isCashier: true},
		{email: "seller@burza.cz", username: "seller", name: "Prodejce Jan", password: "heslo123", isCashier: false},
		{email: "buyer@burza.cz", username: "buyer", name: "Kupující Petr", password: "heslo123", isCashier: false},
	}

	var sellerUser *models.Record
	for _, u := range testUsers {
		user, _ := dao.FindAuthRecordByEmail("users", u.email)
		if user == nil {
			user = models.NewRecord(usersColl)
			_ = user.SetUsername(u.username)
			_ = user.SetEmail(u.email)
			_ = user.SetPassword(u.password)
			user.Set("name", u.name)
			user.Set("isCashier", u.isCashier)
			_ = user.SetVerified(true)
			if err := dao.SaveRecord(user); err != nil {
				log.Printf("[SEED] Error creating user %s: %v", u.email, err)
			} else {
				log.Printf("[SEED] Created test user %s (isCashier: %v)", u.email, u.isCashier)
			}
		}
		if u.email == "seller@burza.cz" {
			sellerUser = user
		}
	}

	// 3. Clean up any invalid placeholder books and seed real sample books
	if sellerUser != nil && event != nil {
		booksColl, err := dao.FindCollectionByNameOrId("books")
		if err == nil {
			// Delete books with empty seller
			emptyBooks, _ := dao.FindRecordsByExpr("books", dbx.HashExp{"seller": ""})
			for _, eb := range emptyBooks {
				_ = dao.DeleteRecord(eb)
			}

			var count int
			_ = dao.DB().Select("count(*)").From("books").Row(&count)
			if count == 0 {
				sampleBooks := []struct {
					code  string
					title string
					price float64
					bgCol color.RGBA
				}{
					{code: "MAT-GYMN-01", title: "Matematika pro gymnázia", price: 150, bgCol: color.RGBA{R: 41, G: 128, B: 185, A: 255}},
					{code: "FYZ-SBIRKA-02", title: "Sbírka úloh z fyziky", price: 120, bgCol: color.RGBA{R: 39, G: 174, B: 96, A: 255}},
					{code: "DEJ-PREHLED-03", title: "Dějiny novověku", price: 200, bgCol: color.RGBA{R: 192, G: 57, B: 43, A: 255}},
					{code: "CJ-LITERATURA-04", title: "Literatura pro SŠ", price: 180, bgCol: color.RGBA{R: 142, G: 68, B: 173, A: 255}},
				}

				for _, sb := range sampleBooks {
					coverBytes, fileName, err := generateDummyCoverBytes(sb.title, "Autor", sb.bgCol)
					if err != nil {
						continue
					}
					book := models.NewRecord(booksColl)
					book.Set("code", sb.code)
					book.Set("seller", sellerUser.Id)
					book.Set("event", event.Id)
					book.Set("price", sb.price)
					book.Set("status", "available")
					book.Set("photo", fileName)
					if err := dao.SaveRecord(book); err != nil {
						log.Printf("[SEED] Error creating sample book %s: %v", sb.code, err)
					} else {
						// Write cover image to disk in storage directory
						destDir := filepath.Join(app.DataDir(), "storage", book.BaseFilesPath())
						_ = os.MkdirAll(destDir, 0755)
						_ = os.WriteFile(filepath.Join(destDir, fileName), coverBytes, 0644)
						attrsJSON := fmt.Sprintf(`{"user.cache_control":"","user.content_disposition":"inline","user.content_encoding":"","user.content_language":"","user.content_type":"image/jpeg","user.metadata":{"original-filename":"%s"},"md5":""}`, fileName)
						_ = os.WriteFile(filepath.Join(destDir, fileName+".attrs"), []byte(attrsJSON), 0644)
						log.Printf("[SEED] Created sample book %s (%s, %.0f Kč)", sb.code, sb.title, sb.price)
					}
				}
			}

			// Ensure all existing books in storage have .attrs so thumbnails can be served
			allBooks, _ := dao.FindRecordsByExpr("books", dbx.NewExp("photo != ''"))
			for _, b := range allBooks {
				photo := b.GetString("photo")
				if photo != "" {
					dir := filepath.Join(app.DataDir(), "storage", b.BaseFilesPath())
					attrsFile := filepath.Join(dir, photo+".attrs")
					if _, err := os.Stat(attrsFile); os.IsNotExist(err) {
						attrsJSON := fmt.Sprintf(`{"user.cache_control":"","user.content_disposition":"inline","user.content_encoding":"","user.content_language":"","user.content_type":"image/jpeg","user.metadata":{"original-filename":"%s"},"md5":""}`, photo)
						_ = os.WriteFile(attrsFile, []byte(attrsJSON), 0644)
					}
				}
			}
		}
	}

	return nil
}

// registerApiEndpoints registers custom API routes
func registerApiEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {
	// POST /api/checkout - Atomic book checkout reservation
	e.Router.POST("/api/checkout", func(c echo.Context) error {
		authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
		if authRecord == nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "Přihlášení je vyžadováno.")
		}

		var req struct {
			BookIds []string `json:"bookIds"`
		}
		if err := c.Bind(&req); err != nil || len(req.BookIds) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Musíte uvést alespoň jednu knihu.")
		}

		var expiresAt string
		err := app.Dao().RunInTransaction(func(txDao *daos.Dao) error {
			now := time.Now().UTC()
			expiresTime := now.Add(15 * time.Minute)
			expiresAt = expiresTime.Format("2006-01-02 15:04:05.000Z")

			for _, bookId := range req.BookIds {
				book, err := txDao.FindRecordById("books", bookId)
				if err != nil || book == nil {
					return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Kniha s ID '%s' nebyla nalezena.", bookId))
				}

				if book.GetString("status") != "available" {
					return echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("Kniha '%s' již není dostupná (stav: %s).", book.GetString("code"), book.GetString("status")))
				}

				if book.GetString("seller") == authRecord.Id {
					return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Nemůžete zakoupit svoji vlastní knihu (%s).", book.GetString("code")))
				}

				book.Set("status", "checkout")
				book.Set("buyer", authRecord.Id)
				book.Set("checkoutExpiresAt", expiresAt)
				if err := txDao.SaveRecord(book); err != nil {
					return fmt.Errorf("chyba při ukládání knihy: %w", err)
				}
			}

			// Add books to user's buy relation
			existingBuy := authRecord.GetStringSlice("buy")
			buyMap := make(map[string]bool)
			for _, b := range existingBuy {
				buyMap[b] = true
			}
			for _, b := range req.BookIds {
				buyMap[b] = true
			}
			updatedBuy := make([]string, 0, len(buyMap))
			for b := range buyMap {
				updatedBuy = append(updatedBuy, b)
			}
			authRecord.Set("buy", updatedBuy)
			if err := txDao.SaveRecord(authRecord); err != nil {
				return fmt.Errorf("chyba při aktualizaci košíku uživatele: %w", err)
			}

			return nil
		})

		if err != nil {
			if httpErr, ok := err.(*echo.HTTPError); ok {
				return httpErr
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, map[string]any{
			"success":   true,
			"buyerId":   authRecord.Id,
			"expiresAt": expiresAt,
			"count":     len(req.BookIds),
		})
	}, apis.ActivityLogger(app), apis.RequireRecordAuth("users"))

	// POST /api/checkout/cancel - Cancel reservation of books
	e.Router.POST("/api/checkout/cancel", func(c echo.Context) error {
		authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
		if authRecord == nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "Přihlášení je vyžadováno.")
		}

		var req struct {
			BookIds []string `json:"bookIds"`
		}
		_ = c.Bind(&req)

		err := app.Dao().RunInTransaction(func(txDao *daos.Dao) error {
			var booksToCancel []*models.Record
			if len(req.BookIds) > 0 {
				for _, id := range req.BookIds {
					b, err := txDao.FindRecordById("books", id)
					if err == nil && b != nil && b.GetString("buyer") == authRecord.Id && b.GetString("status") == "checkout" {
						booksToCancel = append(booksToCancel, b)
					}
				}
			} else {
				// Cancel all books reserved by this user
				books, err := txDao.FindRecordsByExpr("books", dbx.HashExp{
					"buyer":  authRecord.Id,
					"status": "checkout",
				})
				if err == nil {
					booksToCancel = books
				}
			}

			for _, b := range booksToCancel {
				// Cancel any pending payment associated with this book
				pendingPayments, _ := txDao.FindRecordsByExpr("payments", dbx.And(
					dbx.HashExp{"status": "pending"},
					dbx.NewExp("books LIKE {:bId}", dbx.Params{"bId": "%" + b.Id + "%"}),
				))
				for _, pp := range pendingPayments {
					pp.Set("status", "cancelled")
					_ = txDao.SaveRecord(pp)
				}

				b.Set("status", "available")
				b.Set("buyer", "")
				b.Set("checkoutExpiresAt", "")
				_ = txDao.SaveRecord(b)
			}


			// Clean user buy relation
			cancelIds := make(map[string]bool)
			for _, b := range booksToCancel {
				cancelIds[b.Id] = true
			}
			currentBuy := authRecord.GetStringSlice("buy")
			newBuy := make([]string, 0, len(currentBuy))
			for _, id := range currentBuy {
				if !cancelIds[id] {
					newBuy = append(newBuy, id)
				}
			}
			authRecord.Set("buy", newBuy)
			_ = txDao.SaveRecord(authRecord)

			return nil
		})

		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, map[string]any{"success": true})
	}, apis.ActivityLogger(app), apis.RequireRecordAuth("users"))

	// GET /api/cashier/buyer-cart - Cashier scans buyer Data Matrix and gets their checkout books
	e.Router.GET("/api/cashier/buyer-cart", func(c echo.Context) error {
		authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return echo.NewHTTPError(http.StatusForbidden, "Pouze pokladní má přístup k této funkci.")
		}

		buyerId := strings.TrimSpace(c.QueryParam("buyerId"))
		if buyerId == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Chybí buyerId.")
		}

		buyer, err := app.Dao().FindRecordById("users", buyerId)
		if err != nil || buyer == nil {
			return echo.NewHTTPError(http.StatusNotFound, "Kupující s tímto kódem nebyl nalezen.")
		}

		// Find all books currently in checkout for this buyer
		books, err := app.Dao().FindRecordsByExpr("books", dbx.HashExp{
			"buyer":  buyerId,
			"status": "checkout",
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		type bookItem struct {
			Id             string  `json:"id"`
			Code           string  `json:"code"`
			Price          float64 `json:"price"`
			Photo          string  `json:"photo"`
			CollectionId   string  `json:"collectionId"`
			CollectionName string  `json:"collectionName"`
		}

		items := make([]bookItem, 0, len(books))
		var totalAmount float64
		for _, b := range books {
			p := b.GetFloat("price")
			totalAmount += p
			items = append(items, bookItem{
				Id:             b.Id,
				Code:           b.GetString("code"),
				Price:          p,
				Photo:          b.GetString("photo"),
				CollectionId:   b.Collection().Id,
				CollectionName: "books",
			})
		}


		// Also get active event bank account info
		activeEvent, _ := app.Dao().FindFirstRecordByData("events", "active", true)

		return c.JSON(http.StatusOK, map[string]any{
			"buyer": map[string]any{
				"id":    buyer.Id,
				"name":  buyer.GetString("name"),
				"email": buyer.GetString("email"),
			},
			"books":       items,
			"totalAmount": totalAmount,
			"event":       activeEvent,
		})
	}, apis.ActivityLogger(app), apis.RequireRecordAuth("users"))

	// POST /api/cashier/confirm-cash - Confirm cash payment at POS
	e.Router.POST("/api/cashier/confirm-cash", func(c echo.Context) error {
		authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return echo.NewHTTPError(http.StatusForbidden, "Pouze pokladní má oprávnění potvrdit platbu.")
		}

		var req struct {
			BuyerId string   `json:"buyerId"`
			BookIds []string `json:"bookIds"`
		}
		if err := c.Bind(&req); err != nil || req.BuyerId == "" || len(req.BookIds) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Chybí buyerId nebo seznam knih.")
		}

		var createdPayment *models.Record
		err := app.Dao().RunInTransaction(func(txDao *daos.Dao) error {
			vsLock.Lock()
			defer vsLock.Unlock()

			var maxVS int
			_ = txDao.DB().Select("COALESCE(MAX(variableSymbol), 10000)").From("payments").Row(&maxVS)
			nextVS := maxVS + 1

			var totalAmount float64
			for _, bookId := range req.BookIds {
				b, err := txDao.FindRecordById("books", bookId)
				if err != nil || b == nil {
					return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Kniha '%s' nebyla nalezena.", bookId))
				}
				totalAmount += b.GetFloat("price")
				b.Set("status", "bought")
				b.Set("checkoutExpiresAt", "")
				if err := txDao.SaveRecord(b); err != nil {
					return fmt.Errorf("chyba při změně stavu knihy: %w", err)
				}
			}

			// Clean buyer buy relation
			if buyer, err := txDao.FindRecordById("users", req.BuyerId); err == nil {
				cleared := make(map[string]bool)
				for _, id := range req.BookIds {
					cleared[id] = true
				}
				currentBuy := buyer.GetStringSlice("buy")
				newBuy := make([]string, 0, len(currentBuy))
				for _, id := range currentBuy {
					if !cleared[id] {
						newBuy = append(newBuy, id)
					}
				}
				buyer.Set("buy", newBuy)
				_ = txDao.SaveRecord(buyer)
			}

			// If there were pending QR payments for these books, mark them cancelled so no orphans remain
			for _, bookId := range req.BookIds {
				pendingPayments, _ := txDao.FindRecordsByExpr("payments", dbx.And(
					dbx.HashExp{"status": "pending"},
					dbx.NewExp("books LIKE {:bId}", dbx.Params{"bId": "%" + bookId + "%"}),
				))
				for _, pp := range pendingPayments {
					pp.Set("status", "cancelled")
					_ = txDao.SaveRecord(pp)
				}
			}


			paymentsColl, err := txDao.FindCollectionByNameOrId("payments")
			if err != nil {
				return err
			}

			createdPayment = models.NewRecord(paymentsColl)
			createdPayment.Set("variableSymbol", nextVS)
			createdPayment.Set("buyer", req.BuyerId)
			createdPayment.Set("books", req.BookIds)
			createdPayment.Set("totalAmount", totalAmount)
			createdPayment.Set("method", "cash")
			createdPayment.Set("status", "completed")
			createdPayment.Set("cashier", authRecord.Id)

			if err := txDao.SaveRecord(createdPayment); err != nil {
				return fmt.Errorf("chyba při ukládání platby: %w", err)
			}

			return nil
		})

		if err != nil {
			if httpErr, ok := err.(*echo.HTTPError); ok {
				return httpErr
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, map[string]any{
			"success": true,
			"payment": createdPayment,
		})
	}, apis.ActivityLogger(app), apis.RequireRecordAuth("users"))

	// POST /api/cashier/create-qr-payment - Create pending QR payment with sequential VS
	e.Router.POST("/api/cashier/create-qr-payment", func(c echo.Context) error {
		authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return echo.NewHTTPError(http.StatusForbidden, "Pouze pokladní má oprávnění vytvořit QR platbu.")
		}

		var req struct {
			BuyerId string   `json:"buyerId"`
			BookIds []string `json:"bookIds"`
		}
		if err := c.Bind(&req); err != nil || req.BuyerId == "" || len(req.BookIds) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Chybí buyerId nebo seznam knih.")
		}

		var createdPayment *models.Record
		err := app.Dao().RunInTransaction(func(txDao *daos.Dao) error {
			vsLock.Lock()
			defer vsLock.Unlock()

			var maxVS int
			_ = txDao.DB().Select("COALESCE(MAX(variableSymbol), 10000)").From("payments").Row(&maxVS)
			nextVS := maxVS + 1

			var totalAmount float64
			for _, bookId := range req.BookIds {
				b, err := txDao.FindRecordById("books", bookId)
				if err != nil || b == nil {
					return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Kniha '%s' nebyla nalezena.", bookId))
				}
				totalAmount += b.GetFloat("price")
				// Clear expiration timer while awaiting bank transfer so reaper doesn't unreserve
				b.Set("checkoutExpiresAt", "")
				if err := txDao.SaveRecord(b); err != nil {
					return fmt.Errorf("chyba při aktualizaci knihy: %w", err)
				}
			}


			paymentsColl, err := txDao.FindCollectionByNameOrId("payments")
			if err != nil {
				return err
			}

			createdPayment = models.NewRecord(paymentsColl)
			createdPayment.Set("variableSymbol", nextVS)
			createdPayment.Set("buyer", req.BuyerId)
			createdPayment.Set("books", req.BookIds)
			createdPayment.Set("totalAmount", totalAmount)
			createdPayment.Set("method", "qr")
			createdPayment.Set("status", "pending")
			createdPayment.Set("cashier", authRecord.Id)

			if err := txDao.SaveRecord(createdPayment); err != nil {
				return fmt.Errorf("chyba při ukládání platby: %w", err)
			}

			return nil
		})

		if err != nil {
			if httpErr, ok := err.(*echo.HTTPError); ok {
				return httpErr
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		activeEvent, _ := app.Dao().FindFirstRecordByData("events", "active", true)

		return c.JSON(http.StatusOK, map[string]any{
			"success": true,
			"payment": createdPayment,
			"event":   activeEvent,
		})
	}, apis.ActivityLogger(app), apis.RequireRecordAuth("users"))

	// POST /api/cashier/confirm-payment - Confirm a pending QR payment
	e.Router.POST("/api/cashier/confirm-payment", func(c echo.Context) error {
		authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return echo.NewHTTPError(http.StatusForbidden, "Pouze pokladní má oprávnění potvrdit platbu.")
		}

		var req struct {
			PaymentId string `json:"paymentId"`
		}
		if err := c.Bind(&req); err != nil || req.PaymentId == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Chybí paymentId.")
		}

		err := app.Dao().RunInTransaction(func(txDao *daos.Dao) error {
			payment, err := txDao.FindRecordById("payments", req.PaymentId)
			if err != nil || payment == nil {
				return echo.NewHTTPError(http.StatusNotFound, "Platba nebyla nalezena.")
			}

			if payment.GetString("status") != "pending" {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Platba již není ve stavu čekající (aktuální stav: %s).", payment.GetString("status")))
			}

			bookIds := payment.GetStringSlice("books")
			for _, bookId := range bookIds {
				b, err := txDao.FindRecordById("books", bookId)
				if err == nil && b != nil {
					b.Set("status", "bought")
					b.Set("checkoutExpiresAt", "")
					_ = txDao.SaveRecord(b)
				}
			}

			// Clean buyer's buy relation
			buyerId := payment.GetString("buyer")
			if buyer, err := txDao.FindRecordById("users", buyerId); err == nil {
				cleared := make(map[string]bool)
				for _, id := range bookIds {
					cleared[id] = true
				}
				currentBuy := buyer.GetStringSlice("buy")
				newBuy := make([]string, 0, len(currentBuy))
				for _, id := range currentBuy {
					if !cleared[id] {
						newBuy = append(newBuy, id)
					}
				}
				buyer.Set("buy", newBuy)
				_ = txDao.SaveRecord(buyer)
			}

			payment.Set("status", "completed")
			payment.Set("cashier", authRecord.Id)
			if err := txDao.SaveRecord(payment); err != nil {
				return fmt.Errorf("chyba při ukládání platby: %w", err)
			}

			return nil
		})

		if err != nil {
			if httpErr, ok := err.(*echo.HTTPError); ok {
				return httpErr
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, map[string]any{"success": true})
	}, apis.ActivityLogger(app), apis.RequireRecordAuth("users"))
}

// startCheckoutReaper runs every 30 seconds to release expired reservations
func startCheckoutReaper(app *pocketbase.PocketBase) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			nowStr := time.Now().UTC().Format("2006-01-02 15:04:05.000Z")

			records, err := app.Dao().FindRecordsByExpr("books", dbx.And(
				dbx.HashExp{"status": "checkout"},
				dbx.NewExp("checkoutExpiresAt != '' AND checkoutExpiresAt < {:now}", dbx.Params{"now": nowStr}),
			))
			if err != nil || len(records) == 0 {
				continue
			}

			log.Printf("[REAPER] Found %d expired checkout reservations", len(records))
			for _, rec := range records {
				// Don't reap books that have an active pending QR payment
				var pendingCount int
				_ = app.Dao().DB().Select("count(*)").From("payments").Where(dbx.And(
					dbx.HashExp{"status": "pending"},
					dbx.NewExp("books LIKE {:bId}", dbx.Params{"bId": "%" + rec.Id + "%"}),
				)).Row(&pendingCount)

				if pendingCount > 0 {
					continue
				}

				buyerId := rec.GetString("buyer")
				rec.Set("status", "available")
				rec.Set("buyer", "")
				rec.Set("checkoutExpiresAt", "")

				if err := app.Dao().SaveRecord(rec); err != nil {
					log.Printf("[REAPER] Error saving book %s: %v", rec.Id, err)
				}

				if buyerId != "" {
					if buyer, err := app.Dao().FindRecordById("users", buyerId); err == nil {
						buyIds := buyer.GetStringSlice("buy")
						newBuyIds := make([]string, 0, len(buyIds))
						for _, id := range buyIds {
							if id != rec.Id {
								newBuyIds = append(newBuyIds, id)
							}
						}
						buyer.Set("buy", newBuyIds)
						_ = app.Dao().SaveRecord(buyer)
					}
				}
			}
		}
	}()
}
