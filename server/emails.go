package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
	"github.com/pocketbase/pocketbase/tools/types"
)

type EmailTemplateData struct {
	Key               string `json:"key"`
	Name              string `json:"name"`
	Subject           string `json:"subject"`
	BodyIntro         string `json:"bodyIntro"`
	UnacceptedWarning string `json:"unacceptedWarning"`
	PayoutBankNote    string `json:"payoutBankNote"`
	PayoutCashNote    string `json:"payoutCashNote"`
	BodyOutro         string `json:"bodyOutro"`
}

var defaultTemplates = map[string]EmailTemplateData{
	"intake_recap": {
		Key:               "intake_recap",
		Name:              "Potvrzení příjmu učebnic k prodeji",
		Subject:           "Burza učebnic – Rekapitulace nabízených knih ({eventName})",
		BodyIntro:         "Ahoj {name},<br><br>toto je rekapitulace učebnic, které jsi přihlásil/a k prodeji na Burze učebnic ({eventName}).",
		PayoutBankNote:    "Peníze za prodané učebnice ti zašleme po skončení burzy na tvůj bankovní účet: <strong>{iban}</strong>.",
		PayoutCashNote:    "Peníze za prodané učebnice ti budou předány <strong>v hotovosti</strong> spolu s neprodanými učebnicemi.",
		UnacceptedWarning: "Některé z tvých učebnic nebyly přijaty k prodeji do burzy (např. neodpovídaly stavu nebo pravidlům). Pokud si myslíš, že jde o omyl, kontaktuj nás prosím odpovědí na tento e-mail.",
		BodyOutro:         "Po ukončení prodeje na burze ti zašleme závěrečné vyúčtování s přehledem prodaných učebnic.<br><br>V případě dotazů se neváhej obrátit na organizátory odpovědí na tento e-mail.<br><br>Tvůj tým Burzy učebnic",
	},
	"sale_summary": {
		Key:               "sale_summary",
		Name:              "Vyúčtování po skončení prodeje",
		Subject:           "Burza učebnic – Výsledky prodeje a vyúčtování ({eventName})",
		BodyIntro:         "Ahoj {name},<br><br>prodej na Burze učebnic ({eventName}) byl úspěšně ukončen. Zde je rekapitulace tvého prodeje a konečné vyúčtování.",
		PayoutBankNote:    "Částka k vyplacení činí <strong>{totalPayout} Kč</strong>. Peníze ti zašleme na tvůj bankovní účet: <strong>{iban}</strong> do 5 pracovních dnů.",
		PayoutCashNote:    "Částka k vyplacení činí <strong>{totalPayout} Kč</strong>. Peníze ti budou předány <strong>v hotovosti</strong> při vrácení neprodaných učebnic.",
		UnacceptedWarning: "",
		BodyOutro:         "Nezapomeň si prosím v určeném termínu vyzvednout své případné neprodané knihy.<br><br>V případě jakýchkoliv dotazů odpověz na tento e-mail.<br><br>Tvůj tým Burzy učebnic",
	},
}

// LoadEmailTemplate retrieves an email template by key from DB or defaults
func LoadEmailTemplate(app core.App, key string) EmailTemplateData {
	def, exists := defaultTemplates[key]
	if !exists {
		def = EmailTemplateData{Key: key}
	}

	record, err := app.FindFirstRecordByData("email_templates", "key", key)
	if err == nil && record != nil {
		if s := record.GetString("subject"); s != "" {
			def.Subject = s
		}
		if s := record.GetString("name"); s != "" {
			def.Name = s
		}
		if s := record.GetString("bodyIntro"); s != "" {
			def.BodyIntro = s
		}
		if s := record.GetString("unacceptedWarning"); s != "" {
			def.UnacceptedWarning = s
		}
		if s := record.GetString("payoutBankNote"); s != "" {
			def.PayoutBankNote = s
		}
		if s := record.GetString("payoutCashNote"); s != "" {
			def.PayoutCashNote = s
		}
		if s := record.GetString("bodyOutro"); s != "" {
			def.BodyOutro = s
		}
	}

	return def
}

func getBaseAppURL(app core.App) string {
	baseURL := strings.TrimRight(app.Settings().Meta.AppURL, "/")
	if baseURL == "" {
		baseURL = "https://burza.skrat.org"
	}
	return baseURL
}

func getUserDisplayName(user *core.Record) string {
	name := strings.TrimSpace(user.GetString("name"))
	if name != "" {
		return name
	}
	email := user.Email()
	if idx := strings.Index(email, "@"); idx > 0 {
		return email[:idx]
	}
	return email
}

func replaceVariables(text string, vars map[string]string) string {
	result := text
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

func renderBookPhotoHTML(baseURL string, b *core.Record) string {
	photo := b.GetString("photo")
	if photo != "" {
		imgURL := fmt.Sprintf("%s/api/files/books/%s/%s?thumb=120x160", baseURL, b.Id, photo)
		return fmt.Sprintf(`<img src="%s" width="60" height="85" style="display: block; border: 1px solid #000000; object-fit: cover; background-color: #f4f4f5;" alt="Kniha" />`, imgURL)
	}
	return `<div style="width: 60px; height: 85px; background-color: #e4e4e7; border: 1px solid #000000; text-align: center; line-height: 85px; font-size: 10px; color: #71717a; font-weight: bold;">Bez fota</div>`
}

func renderUnacceptedBookPhotoHTML(baseURL string, b *core.Record) string {
	photo := b.GetString("photo")
	if photo != "" {
		imgURL := fmt.Sprintf("%s/api/files/books/%s/%s?thumb=120x160", baseURL, b.Id, photo)
		return fmt.Sprintf(`<img src="%s" width="60" height="85" style="display: block; border: 1px solid #ea580c; object-fit: cover; background-color: #fff7ed;" alt="Kniha" />`, imgURL)
	}
	return `<div style="width: 60px; height: 85px; background-color: #ffedd5; border: 1px solid #ea580c; text-align: center; line-height: 85px; font-size: 10px; color: #9a3412; font-weight: bold;">Bez fota</div>`
}

// wrapEmailHTML wraps content in a responsive, clean, high-contrast email layout
func wrapEmailHTML(title, headerSubtitle, contentHTML string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="cs">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f4f5; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; -webkit-font-smoothing: antialiased; color: #18181b;">
  <table role="presentation" width="100%%" border="0" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5; padding: 24px 12px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" border="0" cellspacing="0" cellpadding="0" style="max-width: 600px; background-color: #ffffff; border: 2px solid #000000; box-shadow: 4px 4px 0px #000000; text-align: left;">
          <!-- Header -->
          <tr>
            <td style="background-color: #000000; color: #ffffff; padding: 20px 24px;">
              <div style="font-size: 11px; font-weight: 800; letter-spacing: 1.5px; text-transform: uppercase; color: #a1a1aa; margin-bottom: 4px;">
                %s
              </div>
              <div style="font-size: 20px; font-weight: 900; letter-spacing: -0.5px; text-transform: uppercase; color: #ffffff;">
                %s
              </div>
            </td>
          </tr>
          <!-- Body Content -->
          <tr>
            <td style="padding: 24px; font-size: 14px; line-height: 1.6; color: #27272a;">
              %s
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="background-color: #fafafa; border-top: 2px solid #000000; padding: 16px 24px; font-size: 11px; color: #71717a; text-align: center;">
              Burza učebnic • Gymnázium Christiana Dopplera<br>
              Kontakt na organizátory: <a href="mailto:burza@skrat.org" style="color: #000000; font-weight: 700;">burza@skrat.org</a>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, html.EscapeString(title), html.EscapeString(headerSubtitle), html.EscapeString(title), contentHTML)
}

// RenderIntakeRecapEmail renders Email 1: Confirmation of books submitted for sale
func RenderIntakeRecapEmail(app core.App, user *core.Record, event *core.Record, acceptedBooks []*core.Record, unacceptedBooks []*core.Record, tmpl *EmailTemplateData) (string, string, error) {
	if tmpl == nil {
		t := LoadEmailTemplate(app, "intake_recap")
		tmpl = &t
	}

	baseURL := getBaseAppURL(app)
	name := getUserDisplayName(user)
	eventName := "Burza učebnic"
	if event != nil && event.GetString("name") != "" {
		eventName = event.GetString("name")
	}

	vars := map[string]string{
		"name":      html.EscapeString(name),
		"eventName": html.EscapeString(eventName),
		"email":     html.EscapeString(user.Email()),
	}

	subject := replaceVariables(tmpl.Subject, vars)

	// Payout variant note: ONLY the selected option!
	var payoutBoxHTML string
	payoutToBank := user.GetBool("payoutToBank")
	iban := strings.TrimSpace(user.GetString("iban"))

	if payoutToBank && iban != "" {
		payoutVars := map[string]string{
			"name":      html.EscapeString(name),
			"eventName": html.EscapeString(eventName),
			"iban":      html.EscapeString(iban),
		}
		payoutNote := replaceVariables(tmpl.PayoutBankNote, payoutVars)
		payoutBoxHTML = fmt.Sprintf(`
      <div style="background-color: #f4f4f5; border: 2px solid #000000; padding: 14px 16px; margin: 18px 0;">
        <div style="font-size: 11px; font-weight: 800; text-transform: uppercase; color: #71717a; margin-bottom: 2px;">Způsob vyplacení výtěžku</div>
        <div style="font-size: 13px; color: #000000;">%s</div>
      </div>`, payoutNote)
	} else {
		payoutVars := map[string]string{
			"name":      html.EscapeString(name),
			"eventName": html.EscapeString(eventName),
		}
		payoutNote := replaceVariables(tmpl.PayoutCashNote, payoutVars)
		payoutBoxHTML = fmt.Sprintf(`
      <div style="background-color: #f4f4f5; border: 2px solid #000000; padding: 14px 16px; margin: 18px 0;">
        <div style="font-size: 11px; font-weight: 800; text-transform: uppercase; color: #71717a; margin-bottom: 2px;">Způsob vyplacení výtěžku</div>
        <div style="font-size: 13px; color: #000000;">%s</div>
      </div>`, payoutNote)
	}

	// Accepted books table (NO CODES - only photo and price)
	var acceptedSectionHTML string
	if len(acceptedBooks) > 0 {
		var sum float64
		for _, b := range acceptedBooks {
			sum += b.GetFloat("price")
		}

		var rowsHTML strings.Builder
		for i, b := range acceptedBooks {
			imgTag := renderBookPhotoHTML(baseURL, b)
			price := b.GetFloat("price")

			borderBottom := "border-bottom: 1px solid #e4e4e7;"
			if i == len(acceptedBooks)-1 {
				borderBottom = ""
			}

			rowsHTML.WriteString(fmt.Sprintf(`
        <tr style="%s">
          <td style="padding: 10px 12px; width: 70px;">
            %s
          </td>
          <td style="padding: 10px 12px; vertical-align: middle;">
            <div style="font-size: 15px; font-weight: 800; color: #000000;">%.0f Kč</div>
            <div style="display: inline-block; background-color: #ecfdf5; border: 1px solid #059669; color: #065f46; font-size: 11px; font-weight: 700; padding: 2px 6px; margin-top: 4px;">
              ✓ Přijato k prodeji
            </div>
          </td>
        </tr>`, borderBottom, imgTag, price))
		}

		acceptedSectionHTML = fmt.Sprintf(`
      <div style="margin: 20px 0 10px 0;">
        <div style="font-size: 12px; font-weight: 900; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 8px; color: #000000;">
          Přijaté učebnice k prodeji (%d ks, celkem %.0f Kč)
        </div>
        <table role="presentation" width="100%%" border="0" cellspacing="0" cellpadding="0" style="border: 2px solid #000000; background-color: #ffffff;">
          %s
        </table>
      </div>`, len(acceptedBooks), sum, rowsHTML.String())
	}

	// Unaccepted books section (if any)
	var unacceptedSectionHTML string
	if len(unacceptedBooks) > 0 {
		var unacceptedRows strings.Builder
		for i, b := range unacceptedBooks {
			imgTag := renderUnacceptedBookPhotoHTML(baseURL, b)
			price := b.GetFloat("price")

			borderBottom := "border-bottom: 1px solid #fed7aa;"
			if i == len(unacceptedBooks)-1 {
				borderBottom = ""
			}

			unacceptedRows.WriteString(fmt.Sprintf(`
        <tr style="%s">
          <td style="padding: 10px 12px; width: 70px;">
            %s
          </td>
          <td style="padding: 10px 12px; vertical-align: middle;">
            <div style="font-size: 14px; font-weight: 700; color: #78350f;">%.0f Kč</div>
            <div style="display: inline-block; background-color: #fee2e2; border: 1px solid #dc2626; color: #991b1b; font-size: 11px; font-weight: 700; padding: 2px 6px; margin-top: 4px;">
              ✗ Nepřijato k prodeji
            </div>
          </td>
        </tr>`, borderBottom, imgTag, price))
		}

		warningText := tmpl.UnacceptedWarning
		if warningText == "" {
			warningText = "Některé z tvých učebnic nebyly přijaty k prodeji do burzy. Pokud si myslíš, že jde o omyl, kontaktuj nás prosím odpovědí na tento e-mail."
		}

		unacceptedSectionHTML = fmt.Sprintf(`
      <div style="margin: 24px 0 10px 0; background-color: #fff7ed; border: 2px solid #ea580c; padding: 14px 16px;">
        <div style="font-size: 12px; font-weight: 900; text-transform: uppercase; color: #9a3412; margin-bottom: 6px;">
          ⚠️ Upozornění: Nepřijaté učebnice (%d ks)
        </div>
        <p style="font-size: 13px; color: #7c2d12; margin: 0 0 12px 0; line-height: 1.5;">
          %s
        </p>
        <table role="presentation" width="100%%" border="0" cellspacing="0" cellpadding="0" style="border: 1px solid #ea580c; background-color: #ffffff;">
          %s
        </table>
      </div>`, len(unacceptedBooks), html.EscapeString(warningText), unacceptedRows.String())
	}

	bodyIntro := replaceVariables(tmpl.BodyIntro, vars)
	bodyOutro := replaceVariables(tmpl.BodyOutro, vars)

	contentHTML := fmt.Sprintf(`
    <div>%s</div>
    %s
    %s
    %s
    <div style="margin-top: 24px; padding-top: 16px; border-top: 1px solid #e4e4e7;">%s</div>`,
		bodyIntro, payoutBoxHTML, acceptedSectionHTML, unacceptedSectionHTML, bodyOutro)

	return subject, wrapEmailHTML("Potvrzení přijatých knih", eventName, contentHTML), nil
}

// RenderSaleSummaryEmail renders Email 2: Final sale summary and payout statement
func RenderSaleSummaryEmail(app core.App, user *core.Record, event *core.Record, acceptedBooks []*core.Record, tmpl *EmailTemplateData) (string, string, error) {
	if tmpl == nil {
		t := LoadEmailTemplate(app, "sale_summary")
		tmpl = &t
	}

	baseURL := getBaseAppURL(app)
	name := getUserDisplayName(user)
	eventName := "Burza učebnic"
	if event != nil && event.GetString("name") != "" {
		eventName = event.GetString("name")
	}

	var soldCount int
	var unsoldCount int
	var totalPayout float64

	for _, b := range acceptedBooks {
		if b.GetString("status") == "bought" {
			soldCount++
			totalPayout += b.GetFloat("price")
		} else {
			unsoldCount++
		}
	}

	payoutToBank := user.GetBool("payoutToBank")
	iban := strings.TrimSpace(user.GetString("iban"))

	vars := map[string]string{
		"name":          html.EscapeString(name),
		"eventName":     html.EscapeString(eventName),
		"email":         html.EscapeString(user.Email()),
		"totalPayout":   fmt.Sprintf("%.0f", totalPayout),
		"soldCount":     fmt.Sprintf("%d", soldCount),
		"acceptedCount": fmt.Sprintf("%d", len(acceptedBooks)),
		"unsoldCount":   fmt.Sprintf("%d", unsoldCount),
		"iban":          html.EscapeString(iban),
	}

	subject := replaceVariables(tmpl.Subject, vars)

	// Summary Highlights Card
	var payoutMethodNote string
	if payoutToBank && iban != "" {
		payoutMethodNote = replaceVariables(tmpl.PayoutBankNote, vars)
	} else {
		payoutMethodNote = replaceVariables(tmpl.PayoutCashNote, vars)
	}

	summaryCardHTML := fmt.Sprintf(`
    <div style="background-color: #f4f4f5; border: 2px solid #000000; padding: 16px 20px; margin: 18px 0;">
      <table role="presentation" width="100%%" border="0" cellspacing="0" cellpadding="0">
        <tr>
          <td style="vertical-align: top; padding-right: 12px;">
            <div style="font-size: 11px; font-weight: 800; text-transform: uppercase; color: #71717a;">Prodáno</div>
            <div style="font-size: 20px; font-weight: 900; color: #000000;">%d z %d ks</div>
          </td>
          <td style="vertical-align: top; text-align: right;">
            <div style="font-size: 11px; font-weight: 800; text-transform: uppercase; color: #71717a;">K vyplacení</div>
            <div style="font-size: 24px; font-weight: 900; color: #059669;">%.0f Kč</div>
          </td>
        </tr>
      </table>
      <div style="margin-top: 12px; padding-top: 10px; border-top: 1px solid #d4d4d8; font-size: 13px; color: #18181b;">
        %s
      </div>
    </div>`, soldCount, len(acceptedBooks), totalPayout, payoutMethodNote)

	// Table of accepted books with sale status (NO CODES)
	var booksTableHTML strings.Builder
	for i, b := range acceptedBooks {
		imgTag := renderBookPhotoHTML(baseURL, b)
		price := b.GetFloat("price")
		isSold := b.GetString("status") == "bought"

		borderBottom := "border-bottom: 1px solid #e4e4e7;"
		if i == len(acceptedBooks)-1 {
			borderBottom = ""
		}

		var statusBadge string
		if isSold {
			statusBadge = `<span style="display: inline-block; background-color: #ecfdf5; border: 1px solid #059669; color: #065f46; font-size: 11px; font-weight: 800; padding: 2px 8px;">✓ PRODÁNO</span>`
		} else {
			statusBadge = `<span style="display: inline-block; background-color: #f4f4f5; border: 1px solid #71717a; color: #52525b; font-size: 11px; font-weight: 700; padding: 2px 8px;">NEPRODÁNO (k vyzvednutí)</span>`
		}

		booksTableHTML.WriteString(fmt.Sprintf(`
      <tr style="%s">
        <td style="padding: 10px 12px; width: 70px;">
          %s
        </td>
        <td style="padding: 10px 12px; vertical-align: middle;">
          <div style="font-size: 15px; font-weight: 800; color: #000000; margin-bottom: 4px;">%.0f Kč</div>
          <div>%s</div>
        </td>
      </tr>`, borderBottom, imgTag, price, statusBadge))
	}

	bodyIntro := replaceVariables(tmpl.BodyIntro, vars)
	bodyOutro := replaceVariables(tmpl.BodyOutro, vars)

	contentHTML := fmt.Sprintf(`
    <div>%s</div>
    %s
    <div style="margin: 20px 0 10px 0;">
      <div style="font-size: 12px; font-weight: 900; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 8px; color: #000000;">
        Přehled tvých učebnic v burze
      </div>
      <table role="presentation" width="100%%" border="0" cellspacing="0" cellpadding="0" style="border: 2px solid #000000; background-color: #ffffff;">
        %s
      </table>
    </div>
    <div style="margin-top: 24px; padding-top: 16px; border-top: 1px solid #e4e4e7;">%s</div>`,
		bodyIntro, summaryCardHTML, booksTableHTML.String(), bodyOutro)

	return subject, wrapEmailHTML("Vyúčtování prodeje", eventName, contentHTML), nil
}

// SendEmail sends an email using PocketBase's configured mailer client
func SendEmail(app core.App, toEmail, toName, subject, htmlBody string) error {
	fromAddr := strings.TrimSpace(app.Settings().Meta.SenderAddress)
	if fromAddr == "" {
		fromAddr = "burza@skrat.org"
	}
	fromName := strings.TrimSpace(app.Settings().Meta.SenderName)
	if fromName == "" {
		fromName = "Burza učebnic"
	}

	msg := &mailer.Message{
		From: mail.Address{
			Address: fromAddr,
			Name:    fromName,
		},
		To: []mail.Address{
			{Address: toEmail, Name: toName},
		},
		Subject: subject,
		HTML:    htmlBody,
	}

	client := app.NewMailClient()
	return client.Send(msg)
}

// EnsureEmailTemplatesSchema initializes the email_templates collection
func EnsureEmailTemplatesSchema(app core.App) error {
	coll, _ := app.FindCollectionByNameOrId("email_templates")
	if coll == nil {
		coll = core.NewBaseCollection("email_templates")
		coll.ListRule = types.Pointer("@request.auth.isCashier = true")
		coll.ViewRule = types.Pointer("@request.auth.isCashier = true")
		coll.CreateRule = types.Pointer("@request.auth.isCashier = true")
		coll.UpdateRule = types.Pointer("@request.auth.isCashier = true")
		coll.DeleteRule = types.Pointer("@request.auth.isCashier = true")

		coll.Fields.Add(
			&core.TextField{Name: "key", Required: true},
			&core.TextField{Name: "name", Required: true},
			&core.TextField{Name: "subject", Required: true},
			&core.TextField{Name: "bodyIntro"},
			&core.TextField{Name: "unacceptedWarning"},
			&core.TextField{Name: "payoutBankNote"},
			&core.TextField{Name: "payoutCashNote"},
			&core.TextField{Name: "bodyOutro"},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		return app.Save(coll)
	}

	var changed bool
	if coll.Fields.GetByName("created") == nil {
		coll.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
		changed = true
	}
	if coll.Fields.GetByName("updated") == nil {
		coll.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
		changed = true
	}
	if coll.ListRule == nil || *coll.ListRule != "@request.auth.isCashier = true" {
		coll.ListRule = types.Pointer("@request.auth.isCashier = true")
		coll.ViewRule = types.Pointer("@request.auth.isCashier = true")
		coll.CreateRule = types.Pointer("@request.auth.isCashier = true")
		coll.UpdateRule = types.Pointer("@request.auth.isCashier = true")
		coll.DeleteRule = types.Pointer("@request.auth.isCashier = true")
		changed = true
	}
	if changed {
		return app.Save(coll)
	}
	return nil
}

// SeedDefaultEmailTemplates populates default email templates if missing
func SeedDefaultEmailTemplates(app core.App) error {
	coll, err := app.FindCollectionByNameOrId("email_templates")
	if err != nil || coll == nil {
		return err
	}

	for key, t := range defaultTemplates {
		existing, _ := app.FindFirstRecordByData("email_templates", "key", key)
		if existing == nil {
			rec := core.NewRecord(coll)
			rec.Set("key", t.Key)
			rec.Set("name", t.Name)
			rec.Set("subject", t.Subject)
			rec.Set("bodyIntro", t.BodyIntro)
			rec.Set("unacceptedWarning", t.UnacceptedWarning)
			rec.Set("payoutBankNote", t.PayoutBankNote)
			rec.Set("payoutCashNote", t.PayoutCashNote)
			rec.Set("bodyOutro", t.BodyOutro)
			if err := app.Save(rec); err != nil {
				log.Printf("[SEED] Failed to seed email template %s: %v", key, err)
			} else {
				log.Printf("[SEED] Seeded default email template: %s", key)
			}
		}
	}
	return nil
}

// registerEmailEndpoints registers the 5 cashier email endpoints
func registerEmailEndpoints(e *core.ServeEvent) {
	// 1. GET /api/admin/emails/stats
	e.Router.GET("/api/admin/emails/stats", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.UnauthorizedError("Pouze pokladník má přístup.", nil)
		}

		event, _ := c.App.FindFirstRecordByData("events", "active", true)
		if event == nil {
			return c.JSON(http.StatusOK, map[string]any{
				"activeEvent":     nil,
				"totalSellers":    0,
				"totalBooks":      0,
				"acceptedBooks":   0,
				"unacceptedBooks": 0,
				"soldBooks":       0,
				"totalPayout":     0,
			})
		}

		books, err := c.App.FindAllRecords("books", dbx.HashExp{"event": event.Id})
		if err != nil {
			return c.InternalServerError("Chyba při načítání knih: "+err.Error(), nil)
		}

		sellersMap := make(map[string]bool)
		var totalBooks int
		var acceptedBooks int
		var unacceptedBooks int
		var soldBooks int
		var totalPayout float64

		for _, b := range books {
			sellerId := b.GetString("seller")
			if sellerId != "" {
				sellersMap[sellerId] = true
			}
			totalBooks++
			if b.GetBool("accepted") {
				acceptedBooks++
				if b.GetString("status") == "bought" {
					soldBooks++
					totalPayout += b.GetFloat("price")
				}
			} else {
				unacceptedBooks++
			}
		}

		return c.JSON(http.StatusOK, map[string]any{
			"activeEvent": map[string]any{
				"id":   event.Id,
				"name": event.GetString("name"),
			},
			"totalSellers":    len(sellersMap),
			"totalBooks":      totalBooks,
			"acceptedBooks":   acceptedBooks,
			"unacceptedBooks": unacceptedBooks,
			"soldBooks":       soldBooks,
			"totalPayout":     totalPayout,
		})
	}).Bind(apis.RequireAuth("users"))

	// 2. POST /api/admin/emails/preview
	e.Router.POST("/api/admin/emails/preview", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.UnauthorizedError("Pouze pokladník má přístup.", nil)
		}

		var req struct {
			Type         string `json:"type"` // "intake_recap" or "sale_summary"
			SellerId     string `json:"sellerId"`
			PayoutOption string `json:"payoutOption"` // "bank" or "cash"
		}
		_ = c.BindBody(&req)
		if req.Type == "" {
			req.Type = "intake_recap"
		}

		event, _ := c.App.FindFirstRecordByData("events", "active", true)

		var targetUser *core.Record
		var userAcceptedBooks []*core.Record
		var userUnacceptedBooks []*core.Record

		if req.SellerId != "" {
			targetUser, _ = c.App.FindRecordById("users", req.SellerId)
		}

		if targetUser != nil && event != nil {
			books, _ := c.App.FindAllRecords("books", dbx.HashExp{"event": event.Id, "seller": targetUser.Id})
			for _, b := range books {
				if b.GetBool("accepted") {
					userAcceptedBooks = append(userAcceptedBooks, b)
				} else {
					userUnacceptedBooks = append(userUnacceptedBooks, b)
				}
			}
		}

		// If no target user or no books, try to find ANY seller with books in the event
		if targetUser == nil && event != nil {
			books, _ := c.App.FindAllRecords("books", dbx.HashExp{"event": event.Id})
			for _, b := range books {
				sId := b.GetString("seller")
				if sId != "" {
					if u, err := c.App.FindRecordById("users", sId); err == nil && u != nil {
						targetUser = u
						break
					}
				}
			}
			if targetUser != nil {
				for _, b := range books {
					if b.GetString("seller") == targetUser.Id {
						if b.GetBool("accepted") {
							userAcceptedBooks = append(userAcceptedBooks, b)
						} else {
							userUnacceptedBooks = append(userUnacceptedBooks, b)
						}
					}
				}
			}
		}

		// Fallback sample data if still nil
		if targetUser == nil {
			usersColl, _ := c.App.FindCollectionByNameOrId("users")
			if usersColl != nil {
				targetUser = core.NewRecord(usersColl)
			} else {
				targetUser = core.NewRecord(core.NewBaseCollection("users"))
			}
			targetUser.Set("name", "Jan Novák")
			targetUser.SetEmail("jan.novak@gchd.cz")
			targetUser.Set("payoutToBank", true)
			targetUser.Set("iban", "CZ6520100000002101234567")

			booksColl, _ := c.App.FindCollectionByNameOrId("books")
			if booksColl == nil {
				booksColl = core.NewBaseCollection("books")
			}
			b1 := core.NewRecord(booksColl)
			b1.Id = "sample_book_1"
			b1.Set("price", 150)
			b1.Set("accepted", true)
			b1.Set("status", "bought")

			b2 := core.NewRecord(booksColl)
			b2.Id = "sample_book_2"
			b2.Set("price", 120)
			b2.Set("accepted", true)
			b2.Set("status", "available")

			b3 := core.NewRecord(booksColl)
			b3.Id = "sample_book_3"
			b3.Set("price", 180)
			b3.Set("accepted", false)

			userAcceptedBooks = []*core.Record{b1, b2}
			userUnacceptedBooks = []*core.Record{b3}
		}

		if req.PayoutOption == "bank" {
			targetUser.Set("payoutToBank", true)
			if targetUser.GetString("iban") == "" {
				targetUser.Set("iban", "CZ6520100000002101234567")
			}
		} else if req.PayoutOption == "cash" {
			targetUser.Set("payoutToBank", false)
		}

		var subject, htmlBody string
		var err error

		if req.Type == "sale_summary" {
			if len(userAcceptedBooks) == 0 {
				booksColl, _ := c.App.FindCollectionByNameOrId("books")
				if booksColl == nil {
					booksColl = core.NewBaseCollection("books")
				}
				b1 := core.NewRecord(booksColl)
				b1.Id = "sample_book_1"
				b1.Set("price", 150)
				b1.Set("accepted", true)
				b1.Set("status", "bought")
				b2 := core.NewRecord(booksColl)
				b2.Id = "sample_book_2"
				b2.Set("price", 120)
				b2.Set("accepted", true)
				b2.Set("status", "available")
				userAcceptedBooks = []*core.Record{b1, b2}
			}
			subject, htmlBody, err = RenderSaleSummaryEmail(c.App, targetUser, event, userAcceptedBooks, nil)
		} else {
			subject, htmlBody, err = RenderIntakeRecapEmail(c.App, targetUser, event, userAcceptedBooks, userUnacceptedBooks, nil)
		}

		if err != nil {
			return c.InternalServerError("Chyba při generování náhledu: "+err.Error(), nil)
		}

		return c.JSON(http.StatusOK, map[string]any{
			"subject": subject,
			"html":    htmlBody,
		})
	}).Bind(apis.RequireAuth("users"))

	// 3. POST /api/admin/emails/send-test
	e.Router.POST("/api/admin/emails/send-test", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.UnauthorizedError("Pouze pokladník má přístup.", nil)
		}

		cashierEmail := strings.TrimSpace(authRecord.Email())
		if cashierEmail == "" {
			return c.BadRequestError("Přihlášený pokladník nemá nastavený e-mail.", nil)
		}

		var req struct {
			Type         string `json:"type"` // "intake_recap" or "sale_summary"
			PayoutOption string `json:"payoutOption"` // "bank" or "cash"
		}
		_ = c.BindBody(&req)
		if req.Type == "" {
			req.Type = "intake_recap"
		}

		event, _ := c.App.FindFirstRecordByData("events", "active", true)

		// Create test user using cashier credentials
		testUser := core.NewRecord(authRecord.Collection())
		testUser.SetEmail(cashierEmail)
		name := getUserDisplayName(authRecord)
		testUser.Set("name", name)

		if req.PayoutOption == "cash" {
			testUser.Set("payoutToBank", false)
		} else if req.PayoutOption == "bank" {
			testUser.Set("payoutToBank", true)
			testUser.Set("iban", "CZ6520100000002101234567")
		} else {
			testUser.Set("payoutToBank", authRecord.GetBool("payoutToBank"))
			testUser.Set("iban", authRecord.GetString("iban"))
			if testUser.GetBool("payoutToBank") && testUser.GetString("iban") == "" {
				testUser.Set("iban", "CZ6520100000002101234567")
			}
		}

		// Try to pick real books for realistic photos, or fallback to sample records
		booksColl, _ := c.App.FindCollectionByNameOrId("books")
		if booksColl == nil {
			booksColl = core.NewBaseCollection("books")
		}
		var userAcceptedBooks []*core.Record
		var userUnacceptedBooks []*core.Record

		if event != nil {
			books, _ := c.App.FindAllRecords("books", dbx.HashExp{"event": event.Id})
			for _, b := range books {
				if b.GetBool("accepted") && len(userAcceptedBooks) < 2 {
					userAcceptedBooks = append(userAcceptedBooks, b)
				} else if !b.GetBool("accepted") && len(userUnacceptedBooks) < 1 {
					userUnacceptedBooks = append(userUnacceptedBooks, b)
				}
			}
		}

		if len(userAcceptedBooks) == 0 {
			b1 := core.NewRecord(booksColl)
			b1.Id = "sample_test_1"
			b1.Set("price", 150)
			b1.Set("accepted", true)
			b1.Set("status", "bought")

			b2 := core.NewRecord(booksColl)
			b2.Id = "sample_test_2"
			b2.Set("price", 120)
			b2.Set("accepted", true)
			b2.Set("status", "available")
			userAcceptedBooks = append(userAcceptedBooks, b1, b2)
		}
		if len(userUnacceptedBooks) == 0 && req.Type == "intake_recap" {
			b3 := core.NewRecord(booksColl)
			b3.Id = "sample_test_3"
			b3.Set("price", 180)
			b3.Set("accepted", false)
			userUnacceptedBooks = append(userUnacceptedBooks, b3)
		}

		var subject, htmlBody string
		var err error

		if req.Type == "sale_summary" {
			subject, htmlBody, err = RenderSaleSummaryEmail(c.App, testUser, event, userAcceptedBooks, nil)
		} else {
			subject, htmlBody, err = RenderIntakeRecapEmail(c.App, testUser, event, userAcceptedBooks, userUnacceptedBooks, nil)
		}

		if err != nil {
			return c.InternalServerError("Chyba při generování testovacího e-mailu: "+err.Error(), nil)
		}

		testSubject := "[TEST] " + subject
		if err := SendEmail(c.App, cashierEmail, name, testSubject, htmlBody); err != nil {
			return c.InternalServerError("Chyba při odesílání e-mailu na "+cashierEmail+": "+err.Error(), nil)
		}

		return c.JSON(http.StatusOK, map[string]any{
			"success":   true,
			"recipient": cashierEmail,
			"subject":   testSubject,
		})
	}).Bind(apis.RequireAuth("users"))

	// 4. POST /api/admin/emails/send-intake-recap
	e.Router.POST("/api/admin/emails/send-intake-recap", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.UnauthorizedError("Pouze pokladník má přístup.", nil)
		}

		event, _ := c.App.FindFirstRecordByData("events", "active", true)
		if event == nil {
			return c.BadRequestError("Nebyla nalezena žádná aktivní burza.", nil)
		}

		books, err := c.App.FindAllRecords("books", dbx.HashExp{"event": event.Id})
		if err != nil {
			return c.InternalServerError("Chyba při načítání knih: "+err.Error(), nil)
		}

		booksBySeller := make(map[string][]*core.Record)
		for _, b := range books {
			sId := b.GetString("seller")
			if sId != "" {
				booksBySeller[sId] = append(booksBySeller[sId], b)
			}
		}

		if len(booksBySeller) == 0 {
			return c.BadRequestError("V aktivní burze nejsou žádné přihlášené knihy k prodeji.", nil)
		}

		tmpl := LoadEmailTemplate(c.App, "intake_recap")

		var totalSent int
		var totalErrors int
		var errorsList []string

		for sellerId, sellerBooks := range booksBySeller {
			user, err := c.App.FindRecordById("users", sellerId)
			if err != nil || user == nil {
				continue
			}
			userEmail := strings.TrimSpace(user.Email())
			if userEmail == "" {
				continue
			}

			var accepted []*core.Record
			var unaccepted []*core.Record
			for _, b := range sellerBooks {
				if b.GetBool("accepted") {
					accepted = append(accepted, b)
				} else {
					unaccepted = append(unaccepted, b)
				}
			}

			subject, htmlBody, err := RenderIntakeRecapEmail(c.App, user, event, accepted, unaccepted, &tmpl)
			if err != nil {
				totalErrors++
				errorsList = append(errorsList, fmt.Sprintf("%s (%s): %v", getUserDisplayName(user), userEmail, err))
				continue
			}

			if err := SendEmail(c.App, userEmail, getUserDisplayName(user), subject, htmlBody); err != nil {
				totalErrors++
				errorsList = append(errorsList, fmt.Sprintf("%s (%s): %v", getUserDisplayName(user), userEmail, err))
			} else {
				totalSent++
			}
		}

		return c.JSON(http.StatusOK, map[string]any{
			"success":     true,
			"totalSent":   totalSent,
			"totalErrors": totalErrors,
			"errors":      errorsList,
		})
	}).Bind(apis.RequireAuth("users"))

	// 5. POST /api/admin/emails/send-sale-summary
	e.Router.POST("/api/admin/emails/send-sale-summary", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil || !authRecord.GetBool("isCashier") {
			return c.UnauthorizedError("Pouze pokladník má přístup.", nil)
		}

		event, _ := c.App.FindFirstRecordByData("events", "active", true)
		if event == nil {
			return c.BadRequestError("Nebyla nalezena žádná aktivní burza.", nil)
		}

		books, err := c.App.FindAllRecords("books", dbx.HashExp{"event": event.Id, "accepted": true})
		if err != nil {
			return c.InternalServerError("Chyba při načítání knih: "+err.Error(), nil)
		}

		booksBySeller := make(map[string][]*core.Record)
		for _, b := range books {
			sId := b.GetString("seller")
			if sId != "" {
				booksBySeller[sId] = append(booksBySeller[sId], b)
			}
		}

		if len(booksBySeller) == 0 {
			return c.BadRequestError("V aktivní burze nejsou žádné přijaté knihy k vyúčtování.", nil)
		}

		tmpl := LoadEmailTemplate(c.App, "sale_summary")

		var totalSent int
		var totalErrors int
		var errorsList []string

		for sellerId, sellerBooks := range booksBySeller {
			user, err := c.App.FindRecordById("users", sellerId)
			if err != nil || user == nil {
				continue
			}
			userEmail := strings.TrimSpace(user.Email())
			if userEmail == "" {
				continue
			}

			subject, htmlBody, err := RenderSaleSummaryEmail(c.App, user, event, sellerBooks, &tmpl)
			if err != nil {
				totalErrors++
				errorsList = append(errorsList, fmt.Sprintf("%s (%s): %v", getUserDisplayName(user), userEmail, err))
				continue
			}

			if err := SendEmail(c.App, userEmail, getUserDisplayName(user), subject, htmlBody); err != nil {
				totalErrors++
				errorsList = append(errorsList, fmt.Sprintf("%s (%s): %v", getUserDisplayName(user), userEmail, err))
			} else {
				totalSent++
			}
		}

		return c.JSON(http.StatusOK, map[string]any{
			"success":     true,
			"totalSent":   totalSent,
			"totalErrors": totalErrors,
			"errors":      errorsList,
		})
	}).Bind(apis.RequireAuth("users"))
}
