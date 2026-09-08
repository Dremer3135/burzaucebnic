package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Global rate limiting lock for Fio Bank API (1 request per 30 seconds limit)
var (
	fioRateLimitMutex  sync.Mutex
	lastFioRequestTime time.Time
	fioBaseURL         = "https://fioapi.fio.cz/v1/rest"
)

const fioCooldownDuration = 30 * time.Second

// loadEnvFiles searches for and loads variables from a .env file into the environment
func loadEnvFiles() {
	candidates := []string{
		".env",
		"../.env",
		"/opt/burzaucebnic/.env",
	}

	for _, path := range candidates {
		if file, err := os.Open(path); err == nil {
			defer file.Close()
			parseEnvReader(file)
			log.Printf("[ENV] Loaded environment variables from %s", path)
			return
		}
	}
}

// parseEnvReader parses key-value pairs from an io.Reader (like a .env file)
func parseEnvReader(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip matching quotes
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}

		// Don't overwrite existing environment variables
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

// FioTransaction represents a normalized incoming transaction from Fio Bank
type FioTransaction struct {
	ID         string    `json:"id"`         // column22 (ID pohybu)
	Date       string    `json:"date"`       // column0 (Datum)
	Amount     float64   `json:"amount"`     // column1 (Objem)
	Currency   string    `json:"currency"`   // column14 (Měna)
	VS         string    `json:"vs"`         // column5 (Variabilní symbol)
	Account    string    `json:"account"`    // column2 (Protiúčet)
	BankCode   string    `json:"bankCode"`   // column3 (Kód banky)
	SenderName string    `json:"senderName"` // column10 / column7
	Message    string    `json:"message"`    // column16 (Zpráva pro příjemce)
}

// checkFioRateLimit checks if enough time has passed since the last Fio request
func checkFioRateLimit() (bool, int) {
	fioRateLimitMutex.Lock()
	defer fioRateLimitMutex.Unlock()

	elapsed := time.Since(lastFioRequestTime)
	if elapsed < fioCooldownDuration {
		remaining := int(math.Ceil((fioCooldownDuration - elapsed).Seconds()))
		return false, remaining
	}
	return true, 0
}

// recordFioRequest updates the timestamp of the last Fio request
func recordFioRequest() {
	fioRateLimitMutex.Lock()
	defer fioRateLimitMutex.Unlock()
	lastFioRequestTime = time.Now()
}

// resetFioRateLimit resets the rate limit timestamp (primarily for tests)
func resetFioRateLimit() {
	fioRateLimitMutex.Lock()
	defer fioRateLimitMutex.Unlock()
	lastFioRequestTime = time.Time{}
}

// Helper to safely extract string value from Fio column
func extractFioString(col map[string]any) string {
	if col == nil {
		return ""
	}
	val, ok := col["value"]
	if !ok || val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		// Integers in JSON often parse as float64
		if v == math.Trunc(v) {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprint(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprint(v)
	}
}

// Helper to safely extract float value from Fio column
func extractFioFloat(col map[string]any) float64 {
	if col == nil {
		return 0
	}
	val, ok := col["value"]
	if !ok || val == nil {
		return 0
	}

	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		return 0
	}
}

// parseFioResponse decodes the raw JSON response from Fio Bank
func parseFioResponse(body []byte) ([]FioTransaction, error) {
	var raw struct {
		AccountStatement struct {
			Info struct {
				AccountId string `json:"accountId"`
				Currency  string `json:"currency"`
			} `json:"info"`
			TransactionList struct {
				Transaction []map[string]map[string]any `json:"transaction"`
			} `json:"transactionList"`
		} `json:"accountStatement"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("neplatný formát odpovědi z Fio banky: %w", err)
	}

	var results []FioTransaction
	for _, t := range raw.AccountStatement.TransactionList.Transaction {
		tx := FioTransaction{
			Date:       extractFioString(t["column0"]),
			Amount:     extractFioFloat(t["column1"]),
			Account:    extractFioString(t["column2"]),
			BankCode:   extractFioString(t["column3"]),
			VS:         extractFioString(t["column5"]),
			SenderName: extractFioString(t["column10"]),
			Currency:   extractFioString(t["column14"]),
			Message:    extractFioString(t["column16"]),
			ID:         extractFioString(t["column22"]),
		}
		if tx.SenderName == "" {
			tx.SenderName = extractFioString(t["column7"])
		}

		results = append(results, tx)
	}

	return results, nil
}

// fetchFioTransactions sends the HTTP GET request to Fio Bank API
func fetchFioTransactions(token string, fromDate, toDate time.Time, client *http.Client) ([]FioTransaction, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("Fio API token nebyl zadán.")
	}

	fromStr := fromDate.Format("2006-01-02")
	toStr := toDate.Format("2006-01-02")
	url := fmt.Sprintf("%s/periods/%s/%s/%s/transactions.json", fioBaseURL, token, fromStr, toStr)

	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("chyba při spojení s Fio bankou: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict { // 409 Conflict
		return nil, errors.New("Fio banka omezuje četnost volání na 1 dotaz za 30 sekund. Počkejte prosím chvíli a zkuste to znovu.")
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errors.New("Fio API token je neplatný nebo vypršela jeho platnost.")
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Fio API vrátilo chybu HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("chyba při čtení odpovědi Fio banky: %w", err)
	}

	return parseFioResponse(bodyBytes)
}

// FioMatchedPaymentInfo contains recap information for a matched payment
type FioMatchedPaymentInfo struct {
	PaymentId        string  `json:"paymentId"`
	VariableSymbol   int     `json:"variableSymbol"`
	TotalAmount      float64 `json:"totalAmount"`
	PaidAmount       float64 `json:"paidAmount"`
	BuyerName        string  `json:"buyerName"`
	BuyerEmail       string  `json:"buyerEmail"`
	FioTransactionId string  `json:"fioTransactionId"`
	SenderName       string  `json:"senderName"`
}

// FioSyncResult is the result returned by SyncFioPayments
type FioSyncResult struct {
	MatchedCount             int                     `json:"matchedCount"`
	TotalPendingBefore       int                     `json:"totalPendingBefore"`
	TotalTransactionsFetched int                     `json:"totalTransactionsFetched"`
	MatchedPayments          []FioMatchedPaymentInfo `json:"matchedPayments"`
	CooldownSeconds          int                     `json:"cooldownSeconds"`
	Message                  string                  `json:"message"`
}

// SyncFioPayments fetches bank transactions and matches them to pending payments
func SyncFioPayments(app core.App, customToken ...string) (*FioSyncResult, error) {
	token := strings.TrimSpace(os.Getenv("FIO_API_TOKEN"))
	if len(customToken) > 0 && customToken[0] != "" {
		token = customToken[0]
	}

	if token == "" {
		return nil, errors.New("FIO_API_TOKEN není nastaven v .env souboru na serveru.")
	}

	// Determine time window: from event start/creation (or last 14 days) to tomorrow
	fromDate := time.Now().AddDate(0, 0, -14)
	toDate := time.Now().AddDate(0, 0, 1)

	if event, _ := app.FindFirstRecordByData("events", "active", true); event != nil {
		createdStr := event.GetString("created")
		if createdStr != "" {
			if t, err := time.Parse("2006-01-02 15:04:05.000Z", createdStr); err == nil {
				fromDate = t.AddDate(0, 0, -1) // 1 day buffer before event creation
			}
		}
	}

	// Fetch bank transactions
	transactions, err := fetchFioTransactions(token, fromDate, toDate, nil)
	recordFioRequest()
	if err != nil {
		return nil, err
	}

	// Find all pending payments
	pendingPayments, err := app.FindAllRecords("payments", dbx.HashExp{"status": "pending"})
	if err != nil {
		return nil, fmt.Errorf("chyba při načítání čekajících plateb: %w", err)
	}

	result := &FioSyncResult{
		TotalPendingBefore:       len(pendingPayments),
		TotalTransactionsFetched: len(transactions),
		CooldownSeconds:          30,
		MatchedPayments:          []FioMatchedPaymentInfo{},
	}

	if len(pendingPayments) == 0 {
		result.Message = "Nebyly nalezeny žádné čekající platby k párování."
		return result, nil
	}

	// Load existing fio_transaction_ids to avoid reusing any bank transaction
	type TxIdRow struct {
		FioTransactionId string `db:"fio_transaction_id"`
	}
	var existingTxRows []TxIdRow
	_ = app.DB().Select("fio_transaction_id").From("payments").Where(dbx.NewExp("fio_transaction_id != ''")).All(&existingTxRows)
	usedTxIDs := make(map[string]bool)
	for _, r := range existingTxRows {
		if r.FioTransactionId != "" {
			usedTxIDs[r.FioTransactionId] = true
		}
	}

	// Filter for incoming bank transactions with positive amounts
	var incomingTxs []FioTransaction
	for _, tx := range transactions {
		if tx.Amount > 0 && tx.ID != "" && !usedTxIDs[tx.ID] {
			incomingTxs = append(incomingTxs, tx)
		}
	}

	// Match pending payments
	for _, payment := range pendingPayments {
		vs := payment.GetInt("variableSymbol")
		if vs <= 0 {
			continue
		}
		totalAmount := payment.GetFloat("totalAmount")
		vsStr := strconv.Itoa(vs)

		// Search for matching incoming transaction
		var matchedTx *FioTransaction
		for i := range incomingTxs {
			tx := &incomingTxs[i]
			if usedTxIDs[tx.ID] {
				continue
			}

			// Clean VS: normalize string (strip leading zeros)
			cleanTxVS := strings.TrimLeft(tx.VS, "0")
			cleanTargetVS := strings.TrimLeft(vsStr, "0")

			if (tx.VS == vsStr || cleanTxVS == cleanTargetVS) && tx.Amount >= totalAmount {
				matchedTx = tx
				break
			}
		}

		if matchedTx != nil {
			// Found matching transaction! Mark as completed inside atomic transaction
			txErr := app.RunInTransaction(func(txApp core.App) error {
				p, err := txApp.FindRecordById("payments", payment.Id)
				if err != nil || p == nil {
					return errors.New("platba nenalezena")
				}
				if p.GetString("status") != "pending" {
					return nil // already updated
				}

				p.Set("status", "completed")
				p.Set("confirmation_type", "automatic")
				p.Set("fio_transaction_id", matchedTx.ID)

				// Mark all books as bought
				bookIds := p.GetStringSlice("books")
				for _, bId := range bookIds {
					b, err := txApp.FindRecordById("books", bId)
					if err == nil && b != nil {
						b.Set("status", "bought")
						_ = txApp.Save(b)
					}
				}

				return txApp.Save(p)
			})

			if txErr == nil {
				usedTxIDs[matchedTx.ID] = true
				result.MatchedCount++

				// Extract buyer info for recap
				buyerName := ""
				buyerEmail := ""
				if buyer, _ := app.FindRecordById("users", payment.GetString("buyer")); buyer != nil {
					buyerName = buyer.GetString("name")
					buyerEmail = buyer.Email()
				}

				result.MatchedPayments = append(result.MatchedPayments, FioMatchedPaymentInfo{
					PaymentId:        payment.Id,
					VariableSymbol:   vs,
					TotalAmount:      totalAmount,
					PaidAmount:       matchedTx.Amount,
					BuyerName:        buyerName,
					BuyerEmail:       buyerEmail,
					FioTransactionId: matchedTx.ID,
					SenderName:       matchedTx.SenderName,
				})
			}
		}
	}

	if result.MatchedCount > 0 {
		result.Message = fmt.Sprintf("Úspěšně spárováno a potvrzeno %d plateb z Fio banky.", result.MatchedCount)
	} else {
		result.Message = "Žádné nové platby k párování nebyly v bance nalezeny."
	}

	return result, nil
}

// registerFioEndpoints registers the cashier sync endpoint
func registerFioEndpoints(e *core.ServeEvent) {
	// POST /api/cashier/sync-fio-payments
	e.Router.POST("/api/cashier/sync-fio-payments", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.ForbiddenError("Pouze pokladník má oprávnění synchronizovat platby.", nil)
		}

		// Enforce 30s rate limit
		ok, remaining := checkFioRateLimit()
		if !ok {
			return c.JSON(http.StatusTooManyRequests, map[string]any{
				"error":             "rate_limit_exceeded",
				"message":           fmt.Sprintf("Fio banka povoluje dotaz pouze jednou za 30 sekund. Počkejte prosím ještě %d sekund.", remaining),
				"retryAfterSeconds": remaining,
			})
		}

		result, err := SyncFioPayments(c.App)
		if err != nil {
			return c.BadRequestError(err.Error(), nil)
		}

		return c.JSON(http.StatusOK, result)
	}).Bind(apis.RequireAuth("users"))
}
