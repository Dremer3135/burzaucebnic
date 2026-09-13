package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/types"
)

// EnsureMoneyReturnsSchema initializes the money_returns collection and ensures books status options
func EnsureMoneyReturnsSchema(app core.App) error {
	usersColl, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return fmt.Errorf("finding users collection: %w", err)
	}

	eventsColl, err := app.FindCollectionByNameOrId("events")
	if err != nil {
		return fmt.Errorf("finding events collection: %w", err)
	}

	booksColl, err := app.FindCollectionByNameOrId("books")
	if err != nil {
		return fmt.Errorf("finding books collection: %w", err)
	}

	// 1. Ensure money_returns collection
	returnsColl, _ := app.FindCollectionByNameOrId("money_returns")
	if returnsColl == nil {
		returnsColl = core.NewBaseCollection("money_returns")
		returnsColl.ListRule = types.Pointer("@request.auth.isCashier = true || @request.auth.id = seller.id")
		returnsColl.ViewRule = types.Pointer("@request.auth.isCashier = true || @request.auth.id = seller.id")
		returnsColl.CreateRule = types.Pointer("@request.auth.isCashier = true")
		returnsColl.UpdateRule = types.Pointer("@request.auth.isCashier = true")
		returnsColl.DeleteRule = types.Pointer("@request.auth.isCashier = true")

		returnsColl.Fields.Add(
			&core.RelationField{
				Name:         "seller",
				Required:     true,
				CollectionId: usersColl.Id,
				MaxSelect:    1,
			},
			&core.RelationField{
				Name:         "event",
				Required:     true,
				CollectionId: eventsColl.Id,
				MaxSelect:    1,
			},
			&core.NumberField{
				Name:     "amount",
				Required: true,
			},
			&core.SelectField{
				Name:      "method",
				Required:  true,
				Values:    []string{"cash", "bank"},
				MaxSelect: 1,
			},
			&core.TextField{
				Name: "fio_transaction_id",
			},
			&core.TextField{
				Name: "variableSymbol",
			},
			&core.RelationField{
				Name:         "cashier",
				CollectionId: usersColl.Id,
				MaxSelect:    1,
			},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)

		if err := app.Save(returnsColl); err != nil {
			return fmt.Errorf("creating money_returns collection: %w", err)
		}
	}

	// 2. Ensure "returned" is included in books status select field
	if booksColl != nil {
		statusField := booksColl.Fields.GetByName("status")
		if statusSelect, ok := statusField.(*core.SelectField); ok {
			hasReturned := false
			for _, val := range statusSelect.Values {
				if val == "returned" {
					hasReturned = true
					break
				}
			}
			if !hasReturned {
				statusSelect.Values = append(statusSelect.Values, "returned")
				if err := app.Save(booksColl); err != nil {
					return fmt.Errorf("updating books status options: %w", err)
				}
			}
		}
	}

	return nil
}

var czechAccountRegex = regexp.MustCompile(`^(\d{1,6}-)?\d{1,10}$`)
var bankCodeRegex = regexp.MustCompile(`^\d{4}$`)

// normalizeCzechAccount removes spaces, leading zeros from prefix and account number,
// returning standard format (e.g. "19-12345678" or "2101234567").
func normalizeCzechAccount(accStr string) string {
	clean := strings.ReplaceAll(strings.TrimSpace(accStr), " ", "")
	if strings.Contains(clean, "-") {
		parts := strings.SplitN(clean, "-", 2)
		prefix := strings.TrimLeft(parts[0], "0")
		account := strings.TrimLeft(parts[1], "0")
		if account == "" {
			account = "0"
		}
		if prefix == "" {
			return account
		}
		return prefix + "-" + account
	}
	account := strings.TrimLeft(clean, "0")
	if account == "" {
		return "0"
	}
	return account
}

// ibanToCzechAccount extracts a domestic Czech account number (with optional prefix)
// and 4-digit bank code from an IBAN or a Czech account string.
func ibanToCzechAccount(ibanStr string) (accountTo string, bankCode string, err error) {
	clean := strings.ToUpper(strings.ReplaceAll(ibanStr, " ", ""))
	clean = strings.TrimPrefix(clean, "IBAN:")

	// Case 1: Already Czech domestic format, e.g. "2101234567/2010" or "19-1234567890/0100"
	if strings.Contains(clean, "/") {
		parts := strings.SplitN(clean, "/", 2)
		rawAcc := strings.TrimSpace(parts[0])
		bCode := strings.TrimSpace(parts[1])
		if len(bCode) == 3 {
			bCode = "0" + bCode
		}
		if !bankCodeRegex.MatchString(bCode) {
			return "", "", fmt.Errorf("neplatný kód banky: %s", bCode)
		}
		normAcc := normalizeCzechAccount(rawAcc)
		if !czechAccountRegex.MatchString(normAcc) {
			return "", "", fmt.Errorf("neplatné číslo účtu: %s", rawAcc)
		}
		return normAcc, bCode, nil
	}

	// Case 2: Czech IBAN (24 characters starting with CZ)
	if strings.HasPrefix(clean, "CZ") && len(clean) == 24 {
		bCode := clean[4:8]
		prefixRaw := clean[8:14]
		accountRaw := clean[14:24]

		if !bankCodeRegex.MatchString(bCode) {
			return "", "", fmt.Errorf("neplatný kód banky v IBAN: %s", bCode)
		}

		normAcc := normalizeCzechAccount(prefixRaw + "-" + accountRaw)
		if !czechAccountRegex.MatchString(normAcc) {
			return "", "", fmt.Errorf("neplatné číslo účtu v IBAN: %s", ibanStr)
		}
		return normAcc, bCode, nil
	}

	return "", "", fmt.Errorf("nepodporovaný formát čísla účtu (očekáván CZ IBAN nebo české číslo účtu): %s", ibanStr)
}

// sanitizeFilenameForHeader removes diacritics and non-ascii characters for clean Content-Disposition filenames
func sanitizeFilenameForHeader(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'á', 'ä':
			b.WriteRune('a')
		case 'č':
			b.WriteRune('c')
		case 'ď':
			b.WriteRune('d')
		case 'é', 'ě', 'ë':
			b.WriteRune('e')
		case 'í', 'ï':
			b.WriteRune('i')
		case 'ň':
			b.WriteRune('n')
		case 'ó', 'ö':
			b.WriteRune('o')
		case 'ř':
			b.WriteRune('r')
		case 'š':
			b.WriteRune('s')
		case 'ť':
			b.WriteRune('t')
		case 'ú', 'ů', 'ü':
			b.WriteRune('u')
		case 'ý', 'ÿ':
			b.WriteRune('y')
		case 'ž':
			b.WriteRune('z')
		case ' ':
			b.WriteRune('_')
		default:
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				b.WriteRune(r)
			}
		}
	}
	res := b.String()
	for strings.Contains(res, "__") {
		res = strings.ReplaceAll(res, "__", "_")
	}
	return strings.Trim(res, "_-")
}

// SellerBalanceSummary holds computed financial balances and book counts for a seller in an event
type SellerBalanceSummary struct {
	SellerId      string  `json:"id"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	PayoutToBank  bool    `json:"payoutToBank"`
	Iban          string  `json:"iban"`
	TotalEarned   float64 `json:"totalEarned"`
	TotalPaid     float64 `json:"totalPaid"`
	TotalUnpaid   float64 `json:"totalUnpaid"`
	PaidInCash    float64 `json:"paidInCash"`
	PaidInBank    float64 `json:"paidInBank"`
	UnsoldCount   int     `json:"unsoldCount"`
	SoldCount     int     `json:"soldCount"`
	ReturnedCount int     `json:"returnedCount"`
}

// CalculateSellerBalance computes current earnings, payouts and remaining unpaid balance for a single seller
func CalculateSellerBalance(app core.App, sellerId string, eventId string) (*SellerBalanceSummary, error) {
	seller, err := app.FindRecordById("users", sellerId)
	if err != nil || seller == nil {
		return nil, fmt.Errorf("prodejce nebyl nalezen")
	}

	summary := &SellerBalanceSummary{
		SellerId:     seller.Id,
		Name:         seller.GetString("name"),
		Email:        seller.GetString("email"),
		PayoutToBank: seller.GetBool("payoutToBank"),
		Iban:         seller.GetString("iban"),
	}

	// Calculate earned amount from sold books
	_ = app.DB().Select("COALESCE(SUM(price), 0)", "COUNT(*)").
		From("books").
		Where(dbx.HashExp{
			"seller": sellerId,
			"event":  eventId,
			"status": "bought",
		}).
		Row(&summary.TotalEarned, &summary.SoldCount)

	// Calculate unsold and returned counts (unsold includes available and checkout)
	_ = app.DB().Select("COUNT(*)").
		From("books").
		Where(dbx.HashExp{
			"seller": sellerId,
			"event":  eventId,
		}).
		AndWhere(dbx.In("status", "available", "checkout")).
		Row(&summary.UnsoldCount)

	_ = app.DB().Select("COUNT(*)").
		From("books").
		Where(dbx.HashExp{
			"seller": sellerId,
			"event":  eventId,
			"status": "returned",
		}).
		Row(&summary.ReturnedCount)

	// Calculate paid amounts from money_returns
	_ = app.DB().Select(
		"COALESCE(SUM(amount), 0)",
		"COALESCE(SUM(CASE WHEN method = 'cash' THEN amount ELSE 0 END), 0)",
		"COALESCE(SUM(CASE WHEN method = 'bank' THEN amount ELSE 0 END), 0)",
	).
		From("money_returns").
		Where(dbx.HashExp{
			"seller": sellerId,
			"event":  eventId,
		}).
		Row(&summary.TotalPaid, &summary.PaidInCash, &summary.PaidInBank)
	summary.TotalUnpaid = math.Max(0, math.Round((summary.TotalEarned-summary.TotalPaid)*100)/100)

	return summary, nil
}

// CalculateAllSellersBalances computes balances for all sellers involved in the active event
func CalculateAllSellersBalances(app core.App, eventId string) ([]*SellerBalanceSummary, error) {
	var bookSellers []string
	_ = app.DB().NewQuery("SELECT DISTINCT seller FROM books WHERE event = {:event}").
		Bind(dbx.Params{"event": eventId}).
		Column(&bookSellers)

	var returnSellers []string
	_ = app.DB().NewQuery("SELECT DISTINCT seller FROM money_returns WHERE event = {:event}").
		Bind(dbx.Params{"event": eventId}).
		Column(&returnSellers)

	sellerIdMap := make(map[string]bool)
	for _, sId := range bookSellers {
		if sId != "" {
			sellerIdMap[sId] = true
		}
	}
	for _, sId := range returnSellers {
		if sId != "" {
			sellerIdMap[sId] = true
		}
	}

	results := make([]*SellerBalanceSummary, 0, len(sellerIdMap))
	for sellerId := range sellerIdMap {
		summary, err := CalculateSellerBalance(app, sellerId, eventId)
		if err == nil && summary != nil {
			results = append(results, summary)
		}
	}

	// Sort alphabetically by name (or email)
	sort.Slice(results, func(i, j int) bool {
		nameI := strings.ToLower(results[i].Name)
		if nameI == "" {
			nameI = strings.ToLower(results[i].Email)
		}
		nameJ := strings.ToLower(results[j].Name)
		if nameJ == "" {
			nameJ = strings.ToLower(results[j].Email)
		}
		return nameI < nameJ
	})

	return results, nil
}

// GenerateFioXmlBatch produces a compliant Fio bank XML batch according to importIB.xsd
func GenerateFioXmlBatch(accountFrom string, summaries []*SellerBalanceSummary, execDate time.Time) ([]byte, error) {
	cleanAccountFrom := strings.TrimSpace(accountFrom)
	if strings.Contains(cleanAccountFrom, "/") {
		cleanAccountFrom = strings.Split(cleanAccountFrom, "/")[0]
	}
	if strings.HasPrefix(strings.ToUpper(cleanAccountFrom), "CZ") {
		acc, _, err := ibanToCzechAccount(cleanAccountFrom)
		if err == nil && acc != "" {
			cleanAccountFrom = acc
		}
	}
	cleanAccountFrom = strings.ReplaceAll(cleanAccountFrom, " ", "")
	if strings.Contains(cleanAccountFrom, "-") {
		parts := strings.Split(cleanAccountFrom, "-")
		cleanAccountFrom = parts[len(parts)-1]
	}
	cleanAccountFrom = strings.TrimLeft(cleanAccountFrom, "0")
	if cleanAccountFrom == "" {
		cleanAccountFrom = "0"
	}

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<Import xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="http://www.fio.cz/schema/importIB.xsd">` + "\n")
	buf.WriteString("  <Orders>\n")

	dateStr := execDate.Format("2006-01-02")
	seqIndex := 1

	for _, s := range summaries {
		if !s.PayoutToBank || strings.TrimSpace(s.Iban) == "" || s.TotalUnpaid <= 0 {
			continue
		}

		accountTo, bankCode, err := ibanToCzechAccount(s.Iban)
		if err != nil {
			continue
		}

		vs := fmt.Sprintf("%02d%02d%04d", execDate.Year()%100, int(execDate.Month()), seqIndex)
		seqIndex++

		// Clean and escape fields
		sellerName := strings.TrimSpace(s.Name)
		if sellerName == "" {
			sellerName = strings.Split(s.Email, "@")[0]
		}
		// Collapse whitespace for XML xs:token schema compliance
		sellerName = strings.Join(strings.Fields(sellerName), " ")
		cleanEmail := strings.TrimSpace(s.Email)

		msg := fmt.Sprintf("Burza ucebnic - Vyplata: %s", sellerName)
		if len([]rune(msg)) > 140 {
			msg = string([]rune(msg)[:140])
		}

		comment := fmt.Sprintf("ID: %s %s (%s)", s.SellerId, sellerName, cleanEmail)
		if len([]rune(comment)) > 140 {
			comment = string([]rune(comment)[:140])
		}

		var escapedMsg bytes.Buffer
		_ = xml.EscapeText(&escapedMsg, []byte(msg))

		var escapedComment bytes.Buffer
		_ = xml.EscapeText(&escapedComment, []byte(comment))

		buf.WriteString("    <DomesticTransaction>\n")
		buf.WriteString(fmt.Sprintf("      <accountFrom>%s</accountFrom>\n", html.EscapeString(cleanAccountFrom)))
		buf.WriteString("      <currency>CZK</currency>\n")
		buf.WriteString(fmt.Sprintf("      <amount>%.2f</amount>\n", s.TotalUnpaid))
		buf.WriteString(fmt.Sprintf("      <accountTo>%s</accountTo>\n", html.EscapeString(accountTo)))
		buf.WriteString(fmt.Sprintf("      <bankCode>%s</bankCode>\n", html.EscapeString(bankCode)))
		buf.WriteString(fmt.Sprintf("      <vs>%s</vs>\n", vs))
		buf.WriteString(fmt.Sprintf("      <date>%s</date>\n", dateStr))
		buf.WriteString(fmt.Sprintf("      <messageForRecipient>%s</messageForRecipient>\n", escapedMsg.String()))
		buf.WriteString(fmt.Sprintf("      <comment>%s</comment>\n", escapedComment.String()))
		buf.WriteString("      <paymentType>431001</paymentType>\n")
		buf.WriteString("    </DomesticTransaction>\n")
	}

	buf.WriteString("  </Orders>\n")
	buf.WriteString("</Import>\n")

	return buf.Bytes(), nil
}

// FioMatchedPayoutInfo represents an outgoing bank payment reconciled to a seller
type FioMatchedPayoutInfo struct {
	SellerId         string  `json:"sellerId"`
	SellerName       string  `json:"sellerName"`
	SellerEmail      string  `json:"sellerEmail"`
	Amount           float64 `json:"amount"`
	FioTransactionId string  `json:"fioTransactionId"`
	VariableSymbol   string  `json:"variableSymbol"`
	Account          string  `json:"account"`
	BankCode         string  `json:"bankCode"`
}

// FioPayoutSyncResult contains recap details from SyncFioPayouts
type FioPayoutSyncResult struct {
	MatchedCount             int                    `json:"matchedCount"`
	TotalTransactionsFetched int                    `json:"totalTransactionsFetched"`
	MatchedPayouts           []FioMatchedPayoutInfo `json:"matchedPayouts"`
	CooldownSeconds          int                    `json:"cooldownSeconds"`
	Message                  string                 `json:"message"`
}

// SyncFioPayouts fetches outgoing bank transactions from Fio and matches them to seller unpaid payouts
func SyncFioPayouts(app core.App, customToken ...string) (*FioPayoutSyncResult, error) {
	token := strings.TrimSpace(os.Getenv("FIO_API_TOKEN"))
	if len(customToken) > 0 && customToken[0] != "" {
		token = customToken[0]
	}

	if token == "" {
		return nil, errors.New("FIO_API_TOKEN není nastaven v .env souboru na serveru.")
	}

	activeEvent, err := app.FindFirstRecordByData("events", "active", true)
	if err != nil || activeEvent == nil {
		return nil, errors.New("nebyla nalezena žádná aktivní burza")
	}

	// Time window: from active event creation (or last 30 days) to tomorrow
	fromDate := time.Now().AddDate(0, 0, -30)
	toDate := time.Now().AddDate(0, 0, 1)
	if createdStr := activeEvent.GetString("created"); createdStr != "" {
		for _, layout := range []string{
			"2006-01-02 15:04:05.000Z",
			time.RFC3339,
			"2006-01-02 15:04:05Z",
			"2006-01-02 15:04:05",
		} {
			if t, err := time.Parse(layout, createdStr); err == nil {
				fromDate = t.AddDate(0, 0, -1)
				break
			}
		}
	}

	// Fetch transactions from Fio
	transactions, err := fetchFioTransactions(token, fromDate, toDate, nil)
	recordFioRequest()
	if err != nil {
		return nil, err
	}

	result := &FioPayoutSyncResult{
		TotalTransactionsFetched: len(transactions),
		CooldownSeconds:          30,
		MatchedPayouts:           []FioMatchedPayoutInfo{},
	}

	// Load existing fio_transaction_ids from money_returns and payments to prevent duplicates
	type TxIdRow struct {
		FioTransactionId string `db:"fio_transaction_id"`
	}
	var existingTxRows []TxIdRow
	_ = app.DB().Select("fio_transaction_id").From("money_returns").Where(dbx.NewExp("fio_transaction_id != ''")).All(&existingTxRows)
	var existingPaymentRows []TxIdRow
	_ = app.DB().Select("fio_transaction_id").From("payments").Where(dbx.NewExp("fio_transaction_id != ''")).All(&existingPaymentRows)
	usedTxIDs := make(map[string]bool)
	for _, r := range existingTxRows {
		if r.FioTransactionId != "" {
			usedTxIDs[r.FioTransactionId] = true
		}
	}
	for _, r := range existingPaymentRows {
		if r.FioTransactionId != "" {
			usedTxIDs[r.FioTransactionId] = true
		}
	}

	// Filter outgoing transactions (amount < 0) that have an ID and haven't been reconciled yet
	var outgoingTxs []FioTransaction
	for _, tx := range transactions {
		if tx.Amount < 0 && tx.ID != "" && !usedTxIDs[tx.ID] {
			outgoingTxs = append(outgoingTxs, tx)
		}
	}

	if len(outgoingTxs) == 0 {
		result.Message = "Nebyly nalezeny žádné nové odchozí platby v bance."
		return result, nil
	}

	// Compute current balances of all sellers in active event
	sellerSummaries, err := CalculateAllSellersBalances(app, activeEvent.Id)
	if err != nil {
		return nil, fmt.Errorf("chyba při výpočtu zůstatků prodejců: %w", err)
	}

	// Map of seller by ID and list of candidate bank sellers
	type candidateSeller struct {
		summary   *SellerBalanceSummary
		accountTo string
		bankCode  string
	}
	var candidateSellers []*candidateSeller
	sellerById := make(map[string]*candidateSeller)

	for _, s := range sellerSummaries {
		cs := &candidateSeller{summary: s}
		if s.PayoutToBank && strings.TrimSpace(s.Iban) != "" {
			acc, bCode, err := ibanToCzechAccount(s.Iban)
			if err == nil {
				cs.accountTo = acc
				cs.bankCode = bCode
			}
		}
		sellerById[s.SellerId] = cs
		candidateSellers = append(candidateSellers, cs)
	}

	moneyReturnsColl, err := app.FindCollectionByNameOrId("money_returns")
	if err != nil {
		return nil, fmt.Errorf("kolekce money_returns nebyla nalezena: %w", err)
	}

	// Match outgoing transactions against candidate sellers
	for _, tx := range outgoingTxs {
		if usedTxIDs[tx.ID] {
			continue
		}

		absAmount := math.Abs(tx.Amount)
		var matched *candidateSeller

		// 1. Direct match: Check if transaction comment or message contains seller ID
		combinedText := strings.ToUpper(tx.Comment + " " + tx.Message)
		for _, cs := range candidateSellers {
			upperSellerId := strings.ToUpper(cs.summary.SellerId)
			if strings.Contains(combinedText, "ID: "+upperSellerId) ||
				strings.Contains(combinedText, "ID:"+upperSellerId) ||
				(len(upperSellerId) >= 10 && strings.Contains(combinedText, upperSellerId)) {
				matched = cs
				break
			}
		}

		// 2. Account & Bank Code match with amount tolerance
		if matched == nil {
			normTxAccount := normalizeCzechAccount(tx.Account)
			for _, cs := range candidateSellers {
				if cs.accountTo == "" || cs.bankCode == "" {
					continue
				}
				normCsAccount := normalizeCzechAccount(cs.accountTo)
				if (tx.BankCode == cs.bankCode || tx.BankCode == "") &&
					(normTxAccount == normCsAccount || tx.Account == cs.accountTo) {
					// Check if amount matches unpaid balance or is positive and within balance
					if math.Abs(absAmount-cs.summary.TotalUnpaid) < 0.05 || absAmount <= cs.summary.TotalUnpaid {
						matched = cs
						break
					}
				}
			}
		}

		if matched != nil {
			// Record money_returns payout inside atomic transaction
			txErr := app.RunInTransaction(func(txApp core.App) error {
				retRecord := core.NewRecord(moneyReturnsColl)
				retRecord.Set("seller", matched.summary.SellerId)
				retRecord.Set("event", activeEvent.Id)
				retRecord.Set("amount", absAmount)
				retRecord.Set("method", "bank")
				retRecord.Set("fio_transaction_id", tx.ID)
				retRecord.Set("variableSymbol", tx.VS)
				return txApp.Save(retRecord)
			})

			if txErr == nil {
				usedTxIDs[tx.ID] = true
				result.MatchedCount++
				matched.summary.TotalUnpaid = math.Max(0, matched.summary.TotalUnpaid-absAmount)

				result.MatchedPayouts = append(result.MatchedPayouts, FioMatchedPayoutInfo{
					SellerId:         matched.summary.SellerId,
					SellerName:       matched.summary.Name,
					SellerEmail:      matched.summary.Email,
					Amount:           absAmount,
					FioTransactionId: tx.ID,
					VariableSymbol:   tx.VS,
					Account:          tx.Account,
					BankCode:         tx.BankCode,
				})
			}
		}
	}

	if result.MatchedCount > 0 {
		result.Message = fmt.Sprintf("Úspěšně spárováno %d výplat z Fio banky.", result.MatchedCount)
	} else {
		result.Message = "Žádné nové výplaty k párování nebyly v bance nalezeny."
	}

	return result, nil
}

// registerReturnsEndpoints registers the cashier return and payout endpoints
func registerReturnsEndpoints(e *core.ServeEvent) {
	// GET /api/cashier/returns/sellers
	e.Router.GET("/api/cashier/returns/sellers", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má přístup k této funkci.", nil)
		}

		activeEvent, err := c.App.FindFirstRecordByData("events", "active", true)
		if err != nil || activeEvent == nil {
			return c.JSON(http.StatusOK, []any{})
		}

		summaries, err := CalculateAllSellersBalances(c.App, activeEvent.Id)
		if err != nil {
			return c.InternalServerError(err.Error(), nil)
		}

		return c.JSON(http.StatusOK, summaries)
	}).Bind(apis.RequireAuth("users"))

	// GET /api/cashier/returns/seller-details?id={id}
	e.Router.GET("/api/cashier/returns/seller-details", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má přístup k této funkci.", nil)
		}

		sellerId := strings.TrimSpace(c.Request.URL.Query().Get("id"))
		if sellerId == "" {
			return c.BadRequestError("Chybí ID prodejce.", nil)
		}

		activeEvent, err := c.App.FindFirstRecordByData("events", "active", true)
		if err != nil || activeEvent == nil {
			return c.BadRequestError("Není aktivní žádná burza.", nil)
		}

		seller, err := c.App.FindRecordById("users", sellerId)
		if err != nil || seller == nil {
			return c.NotFoundError("Prodejce nebyl nalezen.", nil)
		}

		balance, err := CalculateSellerBalance(c.App, sellerId, activeEvent.Id)
		if err != nil {
			return c.InternalServerError(err.Error(), nil)
		}

		// Find books for this seller in this event
		books, err := c.App.FindAllRecords("books", dbx.HashExp{
			"seller": sellerId,
			"event":  activeEvent.Id,
		})
		if err != nil {
			return c.InternalServerError(err.Error(), nil)
		}

		type bookItem struct {
			Id             string         `json:"id"`
			Price          float64        `json:"price"`
			Photo          string         `json:"photo"`
			Status         string         `json:"status"`
			Accepted       bool           `json:"accepted"`
			Buyer          map[string]any `json:"buyer,omitempty"`
			CollectionId   string         `json:"collectionId"`
			CollectionName string         `json:"collectionName"`
		}

		var unsoldBooks []bookItem
		var soldBooks []bookItem
		var returnedBooks []bookItem

		for _, b := range books {
			status := b.GetString("status")
			item := bookItem{
				Id:             b.Id,
				Price:          b.GetFloat("price"),
				Photo:          b.GetString("photo"),
				Status:         status,
				Accepted:       b.GetBool("accepted"),
				CollectionId:   b.Collection().Id,
				CollectionName: "books",
			}

			if status == "bought" {
				if buyerId := b.GetString("buyer"); buyerId != "" {
					if buyer, _ := c.App.FindRecordById("users", buyerId); buyer != nil {
						item.Buyer = map[string]any{
							"id":    buyer.Id,
							"name":  buyer.GetString("name"),
							"email": buyer.GetString("email"),
						}
					}
				}
				soldBooks = append(soldBooks, item)
			} else if status == "returned" {
				returnedBooks = append(returnedBooks, item)
			} else {
				// status == "available" or "checkout"
				unsoldBooks = append(unsoldBooks, item)
			}
		}

		// Sort books consistently by ID
		sort.Slice(unsoldBooks, func(i, j int) bool {
			return unsoldBooks[i].Id < unsoldBooks[j].Id
		})
		sort.Slice(soldBooks, func(i, j int) bool {
			return soldBooks[i].Id < soldBooks[j].Id
		})
		sort.Slice(returnedBooks, func(i, j int) bool {
			return returnedBooks[i].Id < returnedBooks[j].Id
		})

		// Find money returns history
		returns, err := c.App.FindAllRecords("money_returns", dbx.HashExp{
			"seller": sellerId,
			"event":  activeEvent.Id,
		})
		if err != nil {
			return c.InternalServerError(err.Error(), nil)
		}

		type moneyReturnItem struct {
			Id               string         `json:"id"`
			Amount           float64        `json:"amount"`
			Method           string         `json:"method"`
			FioTransactionId string         `json:"fio_transaction_id"`
			VariableSymbol   string         `json:"variableSymbol"`
			Cashier          map[string]any `json:"cashier,omitempty"`
			Created          string         `json:"created"`
		}

		history := make([]moneyReturnItem, 0, len(returns))
		for _, r := range returns {
			var cashierInfo map[string]any
			if cashierId := r.GetString("cashier"); cashierId != "" {
				if cashier, _ := c.App.FindRecordById("users", cashierId); cashier != nil {
					cashierInfo = map[string]any{
						"id":    cashier.Id,
						"name":  cashier.GetString("name"),
						"email": cashier.GetString("email"),
					}
				}
			}

			history = append(history, moneyReturnItem{
				Id:               r.Id,
				Amount:           r.GetFloat("amount"),
				Method:           r.GetString("method"),
				FioTransactionId: r.GetString("fio_transaction_id"),
				VariableSymbol:   r.GetString("variableSymbol"),
				Cashier:          cashierInfo,
				Created:          r.GetString("created"),
			})
		}

		// Sort history descending by creation
		sort.Slice(history, func(i, j int) bool {
			return history[i].Created > history[j].Created
		})

		return c.JSON(http.StatusOK, map[string]any{
			"seller": map[string]any{
				"id":           seller.Id,
				"name":         seller.GetString("name"),
				"email":        seller.GetString("email"),
				"payoutToBank": seller.GetBool("payoutToBank"),
				"iban":         seller.GetString("iban"),
			},
			"event": map[string]any{
				"id":   activeEvent.Id,
				"name": activeEvent.GetString("name"),
			},
			"balances":      balance,
			"unsoldBooks":   unsoldBooks,
			"soldBooks":     soldBooks,
			"returnedBooks": returnedBooks,
			"moneyReturns":  history,
		})
	}).Bind(apis.RequireAuth("users"))

	// POST /api/cashier/returns/pay-cash
	e.Router.POST("/api/cashier/returns/pay-cash", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má přístup k této funkci.", nil)
		}

		var req struct {
			SellerId string  `json:"sellerId"`
			Amount   float64 `json:"amount"`
		}
		if err := c.BindBody(&req); err != nil || strings.TrimSpace(req.SellerId) == "" {
			return c.BadRequestError("Chybí ID prodejce.", nil)
		}

		if req.Amount < 0 {
			return c.BadRequestError("Částka k vyplacení nesmí být záporná.", nil)
		}

		activeEvent, err := c.App.FindFirstRecordByData("events", "active", true)
		if err != nil || activeEvent == nil {
			return c.BadRequestError("Není aktivní žádná burza.", nil)
		}

		var createdReturn *core.Record
		var updatedBalance *SellerBalanceSummary
		var payAmount float64

		err = c.App.RunInTransaction(func(txApp core.App) error {
			balance, err := CalculateSellerBalance(txApp, req.SellerId, activeEvent.Id)
			if err != nil {
				return err
			}

			if balance.TotalUnpaid <= 0 {
				return router.NewApiError(http.StatusBadRequest, "Prodejce nemá žádný nevyplacený zůstatek.", nil)
			}

			if req.Amount > balance.TotalUnpaid {
				return router.NewApiError(http.StatusBadRequest, fmt.Sprintf("Požadovaná částka (%.2f Kč) přesahuje zbývající nevyplacený zůstatek (%.2f Kč).", req.Amount, balance.TotalUnpaid), nil)
			}

			payAmount = balance.TotalUnpaid
			if req.Amount > 0 && req.Amount < payAmount {
				payAmount = req.Amount
			}

			returnsColl, err := txApp.FindCollectionByNameOrId("money_returns")
			if err != nil {
				return err
			}

			createdReturn = core.NewRecord(returnsColl)
			createdReturn.Set("seller", req.SellerId)
			createdReturn.Set("event", activeEvent.Id)
			createdReturn.Set("amount", payAmount)
			createdReturn.Set("method", "cash")
			createdReturn.Set("cashier", authRecord.Id)

			if err := txApp.Save(createdReturn); err != nil {
				return fmt.Errorf("chyba při ukládání záznamu o výplatě: %w", err)
			}

			// Recalculate balance
			updatedBalance, err = CalculateSellerBalance(txApp, req.SellerId, activeEvent.Id)
			return err
		})

		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, map[string]any{
			"success":     true,
			"amount":      payAmount,
			"totalUnpaid": updatedBalance.TotalUnpaid,
			"return":      createdReturn,
			"balances":    updatedBalance,
		})
	}).Bind(apis.RequireAuth("users"))

	// GET /api/cashier/payouts/fio-xml
	e.Router.GET("/api/cashier/payouts/fio-xml", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má přístup k této funkci.", nil)
		}

		activeEvent, err := c.App.FindFirstRecordByData("events", "active", true)
		if err != nil || activeEvent == nil {
			return c.BadRequestError("Není aktivní žádná burza.", nil)
		}

		accountFrom := activeEvent.GetString("bankAccount")
		if accountFrom == "" {
			accountFrom = activeEvent.GetString("iban")
		}
		if accountFrom == "" {
			accountFrom = os.Getenv("FIO_ACCOUNT")
		}
		if accountFrom == "" {
			accountFrom = "2101234567"
		}

		sellerId := strings.TrimSpace(c.Request.URL.Query().Get("sellerId"))
		var summaries []*SellerBalanceSummary

		if sellerId != "" {
			summary, err := CalculateSellerBalance(c.App, sellerId, activeEvent.Id)
			if err != nil || summary == nil {
				return c.NotFoundError("Prodejce nebyl nalezen.", nil)
			}
			if !summary.PayoutToBank || strings.TrimSpace(summary.Iban) == "" {
				return c.BadRequestError("Prodejce nemá nastaven bankovní účet (IBAN).", nil)
			}
			if summary.TotalUnpaid <= 0 {
				return c.BadRequestError("Prodejce nemá žádný nevyplacený zůstatek k bankovnímu převodu.", nil)
			}
			summaries = []*SellerBalanceSummary{summary}
		} else {
			allSummaries, err := CalculateAllSellersBalances(c.App, activeEvent.Id)
			if err != nil {
				return c.InternalServerError(err.Error(), nil)
			}
			for _, s := range allSummaries {
				if s.PayoutToBank && strings.TrimSpace(s.Iban) != "" && s.TotalUnpaid > 0 {
					summaries = append(summaries, s)
				}
			}
		}

		xmlBytes, err := GenerateFioXmlBatch(accountFrom, summaries, time.Now())
		if err != nil {
			return c.InternalServerError("Chyba při generování Fio XML: "+err.Error(), err)
		}

		filename := fmt.Sprintf("fio_vyplaty_burza_%s.xml", time.Now().Format("2006-01-02"))
		if sellerId != "" && len(summaries) > 0 {
			cleanName := sanitizeFilenameForHeader(summaries[0].Name)
			if cleanName == "" {
				cleanName = summaries[0].SellerId
			}
			filename = fmt.Sprintf("fio_vyplata_%s_%s.xml", cleanName, time.Now().Format("2006-01-02"))
		}

		c.Response.Header().Set("Content-Type", "application/xml; charset=utf-8")
		c.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		return c.Blob(http.StatusOK, "application/xml; charset=utf-8", xmlBytes)
	}).Bind(apis.RequireAuth("users"))

	// POST /api/cashier/sync-fio-payouts
	e.Router.POST("/api/cashier/sync-fio-payouts", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má přístup k této funkci.", nil)
		}

		ok, remaining := checkFioRateLimit()
		if !ok {
			return c.JSON(http.StatusTooManyRequests, map[string]any{
				"error":             "rate_limit_exceeded",
				"message":           fmt.Sprintf("Fio banka povoluje dotaz pouze jednou za 30 sekund. Počkejte prosím ještě %d sekund.", remaining),
				"retryAfterSeconds": remaining,
			})
		}

		result, err := SyncFioPayouts(c.App)
		if err != nil {
			return c.BadRequestError(err.Error(), nil)
		}

		return c.JSON(http.StatusOK, result)
	}).Bind(apis.RequireAuth("users"))

	// POST /api/cashier/return-books
	e.Router.POST("/api/cashier/return-books", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladní má přístup k této funkci.", nil)
		}

		var req struct {
			BookIds []string `json:"bookIds"`
		}
		if err := c.BindBody(&req); err != nil || len(req.BookIds) == 0 {
			return c.BadRequestError("Musíte vybrat alespoň jednu knihu k vrácení.", nil)
		}

		// Deduplicate and sanitize book IDs
		seen := make(map[string]bool)
		var cleanBookIds []string
		for _, rawId := range req.BookIds {
			cleanId := strings.TrimSpace(rawId)
			if cleanId != "" && !seen[cleanId] {
				seen[cleanId] = true
				cleanBookIds = append(cleanBookIds, cleanId)
			}
		}

		if len(cleanBookIds) == 0 {
			return c.BadRequestError("Nebylo předáno žádné platné ID knihy.", nil)
		}

		returnedIds := make([]string, 0, len(cleanBookIds))
		err := c.App.RunInTransaction(func(txApp core.App) error {
			for _, id := range cleanBookIds {
				book, err := txApp.FindRecordById("books", id)
				if err != nil || book == nil {
					return c.NotFoundError(fmt.Sprintf("Kniha s ID '%s' nebyla nalezena.", id), nil)
				}

				if book.GetString("status") == "bought" {
					return router.NewApiError(http.StatusBadRequest, fmt.Sprintf("Knihu '%s' nelze vrátit, protože již byla prodána.", book.Id), nil)
				}

				book.Set("status", "returned")
				book.Set("buyer", "")
				book.Set("checkoutExpiresAt", "")

				if err := txApp.Save(book); err != nil {
					return fmt.Errorf("chyba při označení knihy '%s' jako vrácené: %w", book.Id, err)
				}
				returnedIds = append(returnedIds, book.Id)
			}
			return nil
		})

		if err != nil {
			return err
		}

		return c.JSON(http.StatusOK, map[string]any{
			"success":         true,
			"count":           len(returnedIds),
			"returnedBookIds": returnedIds,
		})
	}).Bind(apis.RequireAuth("users"))
}
