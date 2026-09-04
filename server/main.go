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

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/types"
)

var vsLock sync.Mutex

func main() {
	app := pocketbase.New()


	// 2. Hook: FFmpeg compression of book photo after upload
	app.OnRecordAfterCreateSuccess("books").BindFunc(func(e *core.RecordEvent) error {
		photoName := e.Record.GetString("photo")
		if photoName == "" {
			return e.Next()
		}
		filePath := filepath.Join(e.App.DataDir(), "storage", e.Record.BaseFilesPath(), photoName)
		go compressImageWithFFmpeg(filePath)
		return e.Next()
	})

	// 3. Schema & Seed setup on startup, custom API routes & reaper
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := ensureSchema(e.App); err != nil {
			log.Printf("[SCHEMA ERROR] %v", err)
			return err
		}

		if err := seedInitialData(e.App); err != nil {
			log.Printf("[SEED ERROR] %v", err)
		}

		registerApiEndpoints(e)
		startCheckoutReaper(e.App)

		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// compressImageWithFFmpeg scales image down to 720p width maintaining aspect ratio and compresses JPEG
func compressImageWithFFmpeg(filePath string) {
	time.Sleep(100 * time.Millisecond)

	if _, err := os.Stat(filePath); err != nil {
		return
	}

	tmpOutput := filePath + ".opt.jpg"
	cmd := exec.Command("ffmpeg", "-y", "-i", filePath, "-vf", "scale='min(720,iw)':-2", "-q:v", "3", tmpOutput)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[FFMPEG COMPRESSION FAILED] %v: %s", err, string(out))
		return
	}

	if err := os.Rename(tmpOutput, filePath); err != nil {
		log.Printf("[FFMPEG RENAME FAILED] %v", err)
	} else {
		log.Printf("[FFMPEG] Successfully optimized %s to 720p", filePath)
	}
}

// ensureSchema initializes collections if they do not exist
func ensureSchema(app core.App) error {
	// 0. Fetch users collection
	usersColl, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return fmt.Errorf("finding users collection: %w", err)
	}

	// 1. Events collection
	eventsColl, _ := app.FindCollectionByNameOrId("events")
	if eventsColl == nil {
		eventsColl = core.NewBaseCollection("events")
		eventsColl.ListRule = types.Pointer("")
		eventsColl.ViewRule = types.Pointer("")
		eventsColl.CreateRule = types.Pointer("@request.auth.isCashier = true")
		eventsColl.UpdateRule = types.Pointer("@request.auth.isCashier = true")
		eventsColl.DeleteRule = types.Pointer("@request.auth.isCashier = true")
		eventsColl.Fields.Add(
			&core.TextField{Name: "name", Required: true},
			&core.BoolField{Name: "active"},
			&core.SelectField{
				Name:      "defaultPage",
				Values:    []string{"sell", "seeprice"},
				MaxSelect: 1,
			},
			&core.TextField{Name: "bankAccount"},
			&core.TextField{Name: "iban"},
			&core.TextField{Name: "currency"},
		)
		if err := app.Save(eventsColl); err != nil {
			return fmt.Errorf("creating events collection: %w", err)
		}
	} else {
		var changed bool
		for _, f := range []string{"sellStart", "sellEnd", "buyStart", "buyEnd"} {
			if eventsColl.Fields.GetByName(f) != nil {
				eventsColl.Fields.RemoveByName(f)
				changed = true
			}
		}
		if eventsColl.Fields.GetByName("defaultPage") == nil {
			eventsColl.Fields.Add(&core.SelectField{
				Name:      "defaultPage",
				Values:    []string{"sell", "seeprice"},
				MaxSelect: 1,
			})
			changed = true
		}
		if changed {
			if err := app.Save(eventsColl); err != nil {
				return fmt.Errorf("updating events collection: %w", err)
			}
		}
	}

	// 2. Books collection
	booksColl, _ := app.FindCollectionByNameOrId("books")
	if booksColl == nil {
		booksColl = core.NewBaseCollection("books")
		booksColl.ListRule = types.Pointer("@request.auth.id = seller.id || @request.auth.isCashier = true")
		booksColl.ViewRule = types.Pointer("@request.auth.id = seller.id || @request.auth.isCashier = true")
		booksColl.CreateRule = types.Pointer("@request.auth.id != '' && @request.auth.id = @request.body.seller")
		booksColl.UpdateRule = types.Pointer("@request.auth.id != '' && (@request.auth.id = seller || @request.auth.isCashier = true)")
		booksColl.DeleteRule = types.Pointer("@request.auth.id != '' && @request.auth.id = seller && status = 'available'")
		booksColl.Fields.Add(
			&core.RelationField{
				Name: "seller", Required: true,
				CollectionId: usersColl.Id, MaxSelect: 1,
			},
			&core.RelationField{
				Name: "buyer",
				CollectionId: usersColl.Id, MaxSelect: 1,
			},
			&core.RelationField{
				Name: "event", Required: true,
				CollectionId: eventsColl.Id, MaxSelect: 1,
			},
			&core.NumberField{Name: "price", Required: true},
			&core.FileField{
				Name: "photo", Required: true,
				MaxSelect: 1,
				MaxSize:   10485760,
				MimeTypes: []string{"image/jpeg", "image/png", "image/webp"},
				Thumbs:    []string{"100x150"},
			},
			&core.SelectField{
				Name: "status",
				Values:    []string{"available", "checkout", "bought"},
				MaxSelect: 1,
			},
			&core.DateField{Name: "checkoutExpiresAt"},
		)
		if err := app.Save(booksColl); err != nil {
			return fmt.Errorf("creating books collection: %w", err)
		}
	} else {
		var changed bool
		if booksColl.Fields.GetByName("code") != nil {
			booksColl.Fields.RemoveByName("code")
			changed = true
		}
		newIndexes := make([]string, 0, len(booksColl.Indexes))
		for _, idx := range booksColl.Indexes {
			if !strings.Contains(idx, "idx_books_code") {
				newIndexes = append(newIndexes, idx)
			} else {
				changed = true
			}
		}
		booksColl.Indexes = newIndexes

		newRule := "@request.auth.id = seller.id || @request.auth.isCashier = true"
		if booksColl.ListRule == nil || *booksColl.ListRule != newRule {
			booksColl.ListRule = types.Pointer(newRule)
			changed = true
		}
		if booksColl.ViewRule == nil || *booksColl.ViewRule != newRule {
			booksColl.ViewRule = types.Pointer(newRule)
			changed = true
		}
		if changed {
			if err := app.Save(booksColl); err != nil {
				return fmt.Errorf("updating books collection: %w", err)
			}
		}
	}

	// 3. Payments collection
	paymentsColl, _ := app.FindCollectionByNameOrId("payments")
	if paymentsColl == nil {
		paymentsColl = core.NewBaseCollection("payments")
		paymentsColl.ListRule = types.Pointer("@request.auth.isCashier = true || @request.auth.id = buyer")
		paymentsColl.ViewRule = types.Pointer("@request.auth.isCashier = true || @request.auth.id = buyer")
		paymentsColl.CreateRule = types.Pointer("@request.auth.isCashier = true")
		paymentsColl.UpdateRule = types.Pointer("@request.auth.isCashier = true")
		paymentsColl.DeleteRule = types.Pointer("@request.auth.isCashier = true")
		paymentsColl.Fields.Add(
			&core.NumberField{Name: "variableSymbol", Required: true},
			&core.RelationField{
				Name: "buyer", Required: true,
				CollectionId: usersColl.Id, MaxSelect: 1,
			},
			&core.RelationField{
				Name: "books", Required: true,
				CollectionId: booksColl.Id, MaxSelect: 999,
			},
			&core.NumberField{Name: "totalAmount", Required: true},
			&core.SelectField{
				Name: "method", Required: true,
				Values: []string{"qr", "cash"}, MaxSelect: 1,
			},
			&core.SelectField{
				Name: "status",
				Values: []string{"pending", "completed", "cancelled"}, MaxSelect: 1,
			},
			&core.RelationField{
				Name: "cashier",
				CollectionId: usersColl.Id, MaxSelect: 1,
			},
		)
		if err := app.Save(paymentsColl); err != nil {
			return fmt.Errorf("creating payments collection: %w", err)
		}
	}

	// 4. Update users collection with name, isCashier, buy fields, and view/list rules for relations
	{
		var changed bool
		if usersColl.Fields.GetByName("name") == nil {
			usersColl.Fields.Add(&core.TextField{
				Name: "name",
			})
			changed = true
		}
		if usersColl.Fields.GetByName("isCashier") == nil {
			usersColl.Fields.Add(&core.BoolField{
				Name: "isCashier",
			})
			changed = true
		}
		if usersColl.Fields.GetByName("buy") == nil {
			usersColl.Fields.Add(&core.RelationField{
				Name:         "buy",
				CollectionId: booksColl.Id,
				MaxSelect:    999,
			})
			changed = true
		}
		if usersColl.ViewRule == nil || *usersColl.ViewRule != "@request.auth.id != ''" {
			usersColl.ViewRule = types.Pointer("@request.auth.id != ''")
			usersColl.ListRule = types.Pointer("@request.auth.id != ''")
			changed = true
		}
		if usersColl.CreateRule == nil || *usersColl.CreateRule != "" {
			usersColl.CreateRule = types.Pointer("")
			changed = true
		}
		if changed {
			if err := app.Save(usersColl); err != nil {
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
func seedInitialData(app core.App) error {
	// 0. Default Superuser / Admin
	superuser, _ := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "ondrej@skrat.org")
	if superuser == nil {
		superuserColl, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
		if err == nil && superuserColl != nil {
			superuser = core.NewRecord(superuserColl)
			superuser.SetEmail("ondrej@skrat.org")
			superuser.SetPassword("burzajeNej67$")
			superuser.SetVerified(true)
			if err := app.Save(superuser); err != nil {
				log.Printf("[SEED] Error creating superuser ondrej@skrat.org: %v", err)
			} else {
				log.Printf("[SEED] Created superuser admin ondrej@skrat.org")
			}
		}
	}

	// 1. Active Event
	event, _ := app.FindFirstRecordByData("events", "active", true)
	if event == nil {
		eventsColl, err := app.FindCollectionByNameOrId("events")
		if err != nil {
			return err
		}
		event = core.NewRecord(eventsColl)
		event.Set("name", "Podzimní burza učebnic 2026")
		event.Set("active", true)
		event.Set("defaultPage", "sell")
		event.Set("bankAccount", "2101234567/2010")
		event.Set("iban", "CZ6520100000002101234567")
		event.Set("currency", "CZK")
		if err := app.Save(event); err != nil {
			log.Printf("[SEED] Failed to create event: %v", err)
		} else {
			log.Printf("[SEED] Created active event: %s", event.GetString("name"))
		}
	} else if event.GetString("defaultPage") == "" {
		event.Set("defaultPage", "sell")
		_ = app.Save(event)
	}

	// 2. Test Users
	usersColl, err := app.FindCollectionByNameOrId("users")
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

	var sellerUser *core.Record
	for _, u := range testUsers {
		user, _ := app.FindAuthRecordByEmail("users", u.email)
		if user == nil {
			user = core.NewRecord(usersColl)
			user.Set("username", u.username)
			user.SetEmail(u.email)
			user.SetPassword(u.password)
			user.Set("name", u.name)
			user.Set("isCashier", u.isCashier)
			user.SetVerified(true)
			if err := app.Save(user); err != nil {
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
		booksColl, err := app.FindCollectionByNameOrId("books")
		if err == nil {
			emptyBooks, _ := app.FindAllRecords("books", dbx.HashExp{"seller": ""})
			for _, eb := range emptyBooks {
				_ = app.Delete(eb)
			}

			var count int
			_ = app.DB().Select("count(*)").From("books").Row(&count)
			if count == 0 {
				sampleBooks := []struct {
					id    string
					title string
					price float64
					bgCol color.RGBA
				}{
					{id: "matgymn00000001", title: "Matematika pro gymnázia", price: 150, bgCol: color.RGBA{R: 41, G: 128, B: 185, A: 255}},
					{id: "fyzsbirka000002", title: "Sbírka úloh z fyziky", price: 120, bgCol: color.RGBA{R: 39, G: 174, B: 96, A: 255}},
					{id: "dejprehled00003", title: "Dějiny novověku", price: 200, bgCol: color.RGBA{R: 192, G: 57, B: 43, A: 255}},
					{id: "cjliterat000004", title: "Literatura pro SŠ", price: 180, bgCol: color.RGBA{R: 142, G: 68, B: 173, A: 255}},
				}

				for _, sb := range sampleBooks {
					coverBytes, fileName, err := generateDummyCoverBytes(sb.title, "Autor", sb.bgCol)
					if err != nil {
						continue
					}
					book := core.NewRecord(booksColl)
					book.Id = sb.id
					book.Set("seller", sellerUser.Id)
					book.Set("event", event.Id)
					book.Set("price", sb.price)
					book.Set("status", "available")
					file, err := filesystem.NewFileFromBytes(coverBytes, fileName)
					if err == nil {
						book.Set("photo", file)
					}
					if err := app.Save(book); err != nil {
						log.Printf("[SEED] Error creating sample book %s: %v", sb.id, err)
					} else {
						log.Printf("[SEED] Created sample book %s (%s, %.0f Kč)", sb.id, sb.title, sb.price)
					}
				}
			}
		}
	}

	return nil
}

// registerApiEndpoints registers custom API routes
func registerApiEndpoints(e *core.ServeEvent) {
	// GET /api/book-price?id={id} - Query book price and status by Data Matrix ID
	e.Router.GET("/api/book-price", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return c.UnauthorizedError("Přihlášení je vyžadováno.", nil)
		}

		id := strings.TrimSpace(c.Request.URL.Query().Get("id"))
		if id == "" {
			return c.BadRequestError("Chybí ID knihy.", nil)
		}

		book, err := c.App.FindRecordById("books", id)
		if err != nil || book == nil {
			return c.NotFoundError("Kniha nebyla nalezena.", nil)
		}

		return c.JSON(http.StatusOK, map[string]any{
			"id":     book.Id,
			"price":  book.GetFloat("price"),
			"status": book.GetString("status"),
		})
	}).Bind(apis.RequireAuth("users"))

	// POST /api/checkout - Atomic book checkout reservation
	e.Router.POST("/api/checkout", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return c.UnauthorizedError("Přihlášení je vyžadováno.", nil)
		}

		var req struct {
			BookIds []string `json:"bookIds"`
		}
		if err := c.BindBody(&req); err != nil || len(req.BookIds) == 0 {
			return c.BadRequestError("Musíte uvést alespoň jednu knihu.", nil)
		}

		var expiresAt string
		err := c.App.RunInTransaction(func(txApp core.App) error {
			now := time.Now().UTC()
			expiresTime := now.Add(15 * time.Minute)
			expiresAt = expiresTime.Format("2006-01-02 15:04:05.000Z")

			for _, bookId := range req.BookIds {
				book, err := txApp.FindRecordById("books", bookId)
				if err != nil || book == nil {
					return c.NotFoundError(fmt.Sprintf("Kniha s ID '%s' nebyla nalezena.", bookId), nil)
				}

				if book.GetString("status") != "available" {
					return router.NewApiError(http.StatusConflict, fmt.Sprintf("Kniha '%s' již není dostupná (stav: %s).", book.Id, book.GetString("status")), nil)
				}

				if book.GetString("seller") == authRecord.Id {
					return c.BadRequestError(fmt.Sprintf("Nemůžete zakoupit svoji vlastní knihu (%s).", book.Id), nil)
				}

				book.Set("status", "checkout")
				book.Set("buyer", authRecord.Id)
				book.Set("checkoutExpiresAt", expiresAt)
				if err := txApp.Save(book); err != nil {
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
			if err := txApp.Save(authRecord); err != nil {
				return fmt.Errorf("chyba při aktualizaci košíku uživatele: %w", err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, map[string]any{
			"success":   true,
			"buyerId":   authRecord.Id,
			"expiresAt": expiresAt,
			"count":     len(req.BookIds),
		})
	}).Bind(apis.RequireAuth("users"))

	// POST /api/checkout/cancel - Cancel reservation of books
	e.Router.POST("/api/checkout/cancel", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return c.UnauthorizedError("Přihlášení je vyžadováno.", nil)
		}

		var req struct {
			BookIds []string `json:"bookIds"`
		}
		_ = c.BindBody(&req)

		err := c.App.RunInTransaction(func(txApp core.App) error {
			var booksToCancel []*core.Record
			if len(req.BookIds) > 0 {
				for _, id := range req.BookIds {
					b, err := txApp.FindRecordById("books", id)
					if err == nil && b != nil && b.GetString("buyer") == authRecord.Id && b.GetString("status") == "checkout" {
						booksToCancel = append(booksToCancel, b)
					}
				}
			} else {
				// Cancel all books reserved by this user
				books, err := txApp.FindAllRecords("books", dbx.HashExp{
					"buyer":  authRecord.Id,
					"status": "checkout",
				})
				if err == nil {
					booksToCancel = books
				}
			}

			for _, b := range booksToCancel {
				// Cancel any pending payment associated with this book
				pendingPayments, _ := txApp.FindAllRecords("payments", dbx.And(
					dbx.HashExp{"status": "pending"},
					dbx.NewExp("books LIKE {:bId}", dbx.Params{"bId": "%" + b.Id + "%"}),
				))
				for _, pp := range pendingPayments {
					pp.Set("status", "cancelled")
					_ = txApp.Save(pp)
				}

				b.Set("status", "available")
				b.Set("buyer", "")
				b.Set("checkoutExpiresAt", "")
				_ = txApp.Save(b)
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
			_ = txApp.Save(authRecord)

			return nil
		})

		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, map[string]any{"success": true})
	}).Bind(apis.RequireAuth("users"))

	// GET /api/cashier/buyer-cart - Cashier scans buyer Data Matrix and gets their checkout books
	e.Router.GET("/api/cashier/buyer-cart", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má přístup k této funkci.", nil)
		}

		buyerId := strings.TrimSpace(c.Request.URL.Query().Get("buyerId"))
		if buyerId == "" {
			return c.BadRequestError("Chybí buyerId.", nil)
		}

		buyer, err := c.App.FindRecordById("users", buyerId)
		if err != nil || buyer == nil {
			return c.NotFoundError("Kupující s tímto kódem nebyl nalezen.", nil)
		}

		// Find all books currently in checkout for this buyer
		books, err := c.App.FindAllRecords("books", dbx.HashExp{
			"buyer":  buyerId,
			"status": "checkout",
		})
		if err != nil {
			return c.InternalServerError(err.Error(), nil)
		}

		type bookItem struct {
			Id             string  `json:"id"`
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
				Price:          p,
				Photo:          b.GetString("photo"),
				CollectionId:   b.Collection().Id,
				CollectionName: "books",
			})
		}

		// Also get active event bank account info
		activeEvent, _ := c.App.FindFirstRecordByData("events", "active", true)

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
	}).Bind(apis.RequireAuth("users"))

	// POST /api/cashier/confirm-cash - Confirm cash payment at POS
	e.Router.POST("/api/cashier/confirm-cash", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má oprávnění potvrdit platbu.", nil)
		}

		var req struct {
			BuyerId string   `json:"buyerId"`
			BookIds []string `json:"bookIds"`
		}
		if err := c.BindBody(&req); err != nil || req.BuyerId == "" || len(req.BookIds) == 0 {
			return c.BadRequestError("Chybí buyerId nebo seznam knih.", nil)
		}

		var createdPayment *core.Record
		err := c.App.RunInTransaction(func(txApp core.App) error {
			vsLock.Lock()
			defer vsLock.Unlock()

			var maxVS int
			_ = txApp.DB().Select("COALESCE(MAX(variableSymbol), 10000)").From("payments").Row(&maxVS)
			nextVS := maxVS + 1

			var totalAmount float64
			for _, bookId := range req.BookIds {
				b, err := txApp.FindRecordById("books", bookId)
				if err != nil || b == nil {
					return c.NotFoundError(fmt.Sprintf("Kniha '%s' nebyla nalezena.", bookId), nil)
				}
				totalAmount += b.GetFloat("price")
				b.Set("status", "bought")
				b.Set("checkoutExpiresAt", "")
				if err := txApp.Save(b); err != nil {
					return fmt.Errorf("chyba při změně stavu knihy: %w", err)
				}
			}

			// Clean buyer buy relation
			if buyer, err := txApp.FindRecordById("users", req.BuyerId); err == nil {
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
				_ = txApp.Save(buyer)
			}

			// If there were pending QR payments for these books, mark them cancelled
			for _, bookId := range req.BookIds {
				pendingPayments, _ := txApp.FindAllRecords("payments", dbx.And(
					dbx.HashExp{"status": "pending"},
					dbx.NewExp("books LIKE {:bId}", dbx.Params{"bId": "%" + bookId + "%"}),
				))
				for _, pp := range pendingPayments {
					pp.Set("status", "cancelled")
					_ = txApp.Save(pp)
				}
			}

			paymentsColl, err := txApp.FindCollectionByNameOrId("payments")
			if err != nil {
				return err
			}

			createdPayment = core.NewRecord(paymentsColl)
			createdPayment.Set("variableSymbol", nextVS)
			createdPayment.Set("buyer", req.BuyerId)
			createdPayment.Set("books", req.BookIds)
			createdPayment.Set("totalAmount", totalAmount)
			createdPayment.Set("method", "cash")
			createdPayment.Set("status", "completed")
			createdPayment.Set("cashier", authRecord.Id)

			if err := txApp.Save(createdPayment); err != nil {
				return fmt.Errorf("chyba při ukládání platby: %w", err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, map[string]any{
			"success": true,
			"payment": createdPayment,
		})
	}).Bind(apis.RequireAuth("users"))

	// POST /api/cashier/create-qr-payment - Create pending QR payment with sequential VS
	e.Router.POST("/api/cashier/create-qr-payment", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má oprávnění vytvořit QR platbu.", nil)
		}

		var req struct {
			BuyerId string   `json:"buyerId"`
			BookIds []string `json:"bookIds"`
		}
		if err := c.BindBody(&req); err != nil || req.BuyerId == "" || len(req.BookIds) == 0 {
			return c.BadRequestError("Chybí buyerId nebo seznam knih.", nil)
		}

		var createdPayment *core.Record
		err := c.App.RunInTransaction(func(txApp core.App) error {
			vsLock.Lock()
			defer vsLock.Unlock()

			var maxVS int
			_ = txApp.DB().Select("COALESCE(MAX(variableSymbol), 10000)").From("payments").Row(&maxVS)
			nextVS := maxVS + 1

			var totalAmount float64
			for _, bookId := range req.BookIds {
				b, err := txApp.FindRecordById("books", bookId)
				if err != nil || b == nil {
					return c.NotFoundError(fmt.Sprintf("Kniha '%s' nebyla nalezena.", bookId), nil)
				}
				totalAmount += b.GetFloat("price")
				// Clear expiration timer while awaiting bank transfer so reaper doesn't unreserve
				b.Set("checkoutExpiresAt", "")
				if err := txApp.Save(b); err != nil {
					return fmt.Errorf("chyba při aktualizaci knihy: %w", err)
				}
			}

			paymentsColl, err := txApp.FindCollectionByNameOrId("payments")
			if err != nil {
				return err
			}

			createdPayment = core.NewRecord(paymentsColl)
			createdPayment.Set("variableSymbol", nextVS)
			createdPayment.Set("buyer", req.BuyerId)
			createdPayment.Set("books", req.BookIds)
			createdPayment.Set("totalAmount", totalAmount)
			createdPayment.Set("method", "qr")
			createdPayment.Set("status", "pending")
			createdPayment.Set("cashier", authRecord.Id)

			if err := txApp.Save(createdPayment); err != nil {
				return fmt.Errorf("chyba při ukládání platby: %w", err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		activeEvent, _ := c.App.FindFirstRecordByData("events", "active", true)

		return c.JSON(http.StatusOK, map[string]any{
			"success": true,
			"payment": createdPayment,
			"event":   activeEvent,
		})
	}).Bind(apis.RequireAuth("users"))

	// POST /api/cashier/confirm-payment - Confirm a pending QR payment
	e.Router.POST("/api/cashier/confirm-payment", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má oprávnění potvrdit platbu.", nil)
		}

		var req struct {
			PaymentId string `json:"paymentId"`
		}
		if err := c.BindBody(&req); err != nil || req.PaymentId == "" {
			return c.BadRequestError("Chybí paymentId.", nil)
		}

		err := c.App.RunInTransaction(func(txApp core.App) error {
			payment, err := txApp.FindRecordById("payments", req.PaymentId)
			if err != nil || payment == nil {
				return c.NotFoundError("Platba nebyla nalezena.", nil)
			}

			if payment.GetString("status") != "pending" {
				return c.BadRequestError(fmt.Sprintf("Platba již není ve stavu čekající (aktuální stav: %s).", payment.GetString("status")), nil)
			}

			bookIds := payment.GetStringSlice("books")
			for _, bookId := range bookIds {
				b, err := txApp.FindRecordById("books", bookId)
				if err == nil && b != nil {
					b.Set("status", "bought")
					b.Set("checkoutExpiresAt", "")
					_ = txApp.Save(b)
				}
			}

			// Clean buyer's buy relation
			buyerId := payment.GetString("buyer")
			if buyer, err := txApp.FindRecordById("users", buyerId); err == nil {
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
				_ = txApp.Save(buyer)
			}

			payment.Set("status", "completed")
			payment.Set("cashier", authRecord.Id)
			if err := txApp.Save(payment); err != nil {
				return fmt.Errorf("chyba při ukládání platby: %w", err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, map[string]any{"success": true})
	}).Bind(apis.RequireAuth("users"))
}

// startCheckoutReaper runs every 30 seconds to release expired reservations
func startCheckoutReaper(app core.App) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			nowStr := time.Now().UTC().Format("2006-01-02 15:04:05.000Z")

			records, err := app.FindAllRecords("books", dbx.And(
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
				_ = app.DB().Select("count(*)").From("payments").Where(dbx.And(
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

				if err := app.Save(rec); err != nil {
					log.Printf("[REAPER] Error saving book %s: %v", rec.Id, err)
				}

				if buyerId != "" {
					if buyer, err := app.FindRecordById("users", buyerId); err == nil {
						buyIds := buyer.GetStringSlice("buy")
						newBuyIds := make([]string, 0, len(buyIds))
						for _, id := range buyIds {
							if id != rec.Id {
								newBuyIds = append(newBuyIds, id)
							}
						}
						buyer.Set("buy", newBuyIds)
						_ = app.Save(buyer)
					}
				}
			}
		}
	}()
}
