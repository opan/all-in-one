package email

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

// Kind is a type of account email.
type Kind string

const (
	KindVerifyEmail     Kind = "verify_email"
	KindTempPassword    Kind = "temp_password"
	KindPasswordChanged Kind = "password_changed"
	KindEmailChanged    Kind = "email_changed"
)

// Data fills the templates. AppName is the app the user came from (e.g.
// Cashflow), empty for aio itself.
type Data struct {
	AppName      string
	Username     string
	Link         string
	TempPassword string
	Minutes      int
	NewEmail     string // masked
	Host         string
}

type emailCopy struct {
	subject string
	// body paragraphs; {{.X}} fields from Data. The temporary password and the
	// link get their own block, so they are not part of these.
	intro  string
	action string // text above the link or password
	outro  string
}

var copies = map[string]map[Kind]emailCopy{
	"en": {
		KindVerifyEmail: {
			subject: "Confirm your email",
			intro:   "Hi {{.Username}}, confirm that this is your email address{{if .AppName}} for the account you use in {{.AppName}}{{end}}. Once it's confirmed, you can reset your password yourself if you forget it.",
			action:  "Open this link within 24 hours:",
			outro:   "If you didn't add this address, ignore this email; nothing changes.",
		},
		KindTempPassword: {
			subject: "Your temporary password",
			intro:   "Hi {{.Username}}, here is a temporary password for {{if .AppName}}the account you use in {{.AppName}}{{else}}your All-in-one account{{end}}.",
			action:  "Log in with it within {{.Minutes}} minutes; you'll then choose a new password:",
			outro:   "Your current password keeps working until you set a new one. If you didn't ask for this, ignore this email.",
		},
		KindPasswordChanged: {
			subject: "Your password was changed",
			intro:   "Hi {{.Username}}, the password for {{if .AppName}}the account you use in {{.AppName}}{{else}}your All-in-one account{{end}} was just changed.",
			outro:   "If this wasn't you, contact the admin right away.",
		},
		KindEmailChanged: {
			subject: "Your email was changed",
			intro:   "Hi {{.Username}}, the email on your All-in-one account was changed to {{.NewEmail}}. Password reset emails now go there.",
			outro:   "If this wasn't you, contact the admin right away.",
		},
	},
	"id": {
		KindVerifyEmail: {
			subject: "Konfirmasi email kamu",
			intro:   "Hai {{.Username}}, konfirmasi bahwa ini alamat email kamu{{if .AppName}} untuk akun yang kamu pakai di {{.AppName}}{{end}}. Setelah dikonfirmasi, kamu bisa mengatur ulang kata sandi sendiri jika lupa.",
			action:  "Buka tautan ini dalam 24 jam:",
			outro:   "Jika kamu tidak menambahkan alamat ini, abaikan email ini; tidak ada yang berubah.",
		},
		KindTempPassword: {
			subject: "Kata sandi sementara kamu",
			intro:   "Hai {{.Username}}, ini kata sandi sementara untuk {{if .AppName}}akun yang kamu pakai di {{.AppName}}{{else}}akun All-in-one kamu{{end}}.",
			action:  "Masuk dengan kata sandi ini dalam {{.Minutes}} menit; setelah itu kamu memilih kata sandi baru:",
			outro:   "Kata sandi lama kamu tetap bisa dipakai sampai kamu membuat yang baru. Jika kamu tidak memintanya, abaikan email ini.",
		},
		KindPasswordChanged: {
			subject: "Kata sandi kamu sudah diganti",
			intro:   "Hai {{.Username}}, kata sandi untuk {{if .AppName}}akun yang kamu pakai di {{.AppName}}{{else}}akun All-in-one kamu{{end}} baru saja diganti.",
			outro:   "Jika bukan kamu yang menggantinya, segera hubungi admin.",
		},
		KindEmailChanged: {
			subject: "Email kamu sudah diganti",
			intro:   "Hai {{.Username}}, email di akun All-in-one kamu diganti menjadi {{.NewEmail}}. Email untuk mengatur ulang kata sandi sekarang dikirim ke sana.",
			outro:   "Jika bukan kamu yang menggantinya, segera hubungi admin.",
		},
	},
}

var managedBy = map[string]string{
	"en": "Your account is managed by All-in-one ({{.Host}}), which is why this email comes from All-in-one.",
	"id": "Akun kamu dikelola oleh All-in-one ({{.Host}}), karena itu email ini dikirim oleh All-in-one.",
}

var htmlLayout = htmltemplate.Must(htmltemplate.New("email").Parse(`<!doctype html>
<html><body style="font-family:system-ui,-apple-system,Segoe UI,sans-serif;color:#17203a;line-height:1.5;max-width:520px;margin:0 auto;padding:24px">
<p>{{.Intro}}</p>
{{if .Action}}<p>{{.Action}}</p>{{end}}
{{if .TempPassword}}<p style="font-family:ui-monospace,Menlo,monospace;font-size:22px;font-weight:700;letter-spacing:2px;background:#eef2ff;padding:12px 16px;display:inline-block">{{.TempPassword}}</p>{{end}}
{{if .Link}}<p><a href="{{.Link}}" style="color:#1f56d6">{{.Link}}</a></p>{{end}}
<p>{{.Outro}}</p>
<p style="color:#5b647a;font-size:13px;margin-top:28px">{{.ManagedBy}}</p>
</body></html>`))

// render builds a message. Text from Data (app name, username) is escaped in
// the HTML part by html/template.
func render(kind Kind, locale string, data Data) (Message, error) {
	if _, ok := copies[locale]; !ok {
		locale = "en"
	}
	c, ok := copies[locale][kind]
	if !ok {
		return Message{}, fmt.Errorf("unknown email kind %q", kind)
	}
	if data.TempPassword != "" {
		data.TempPassword = FormatTempPassword(data.TempPassword)
	}
	fill := func(tpl string) (string, error) {
		t, err := texttemplate.New("").Parse(tpl)
		if err != nil {
			return "", err
		}
		var b bytes.Buffer
		err = t.Execute(&b, data)
		return b.String(), err
	}
	parts := make([]string, 0, 5)
	for _, tpl := range []string{c.subject, c.intro, c.action, c.outro, managedBy[locale]} {
		s, err := fill(tpl)
		if err != nil {
			return Message{}, err
		}
		parts = append(parts, s)
	}
	subject, intro, action, outro, managed := parts[0], parts[1], parts[2], parts[3], parts[4]

	var text strings.Builder
	text.WriteString(intro + "\n\n")
	if action != "" {
		text.WriteString(action + "\n\n")
	}
	if data.TempPassword != "" {
		text.WriteString("    " + data.TempPassword + "\n\n")
	}
	if data.Link != "" {
		text.WriteString(data.Link + "\n\n")
	}
	text.WriteString(outro + "\n\n-- \n" + managed + "\n")

	var html bytes.Buffer
	err := htmlLayout.Execute(&html, struct {
		Intro, Action, TempPassword, Outro, ManagedBy string
		Link                                          htmltemplate.URL
	}{intro, action, data.TempPassword, outro, managed, htmltemplate.URL(data.Link)})
	if err != nil {
		return Message{}, err
	}
	return Message{Subject: subject, Text: text.String(), HTML: html.String()}, nil
}

// FormatTempPassword groups a temporary password in fours for reading:
// "abcdefghjkmn" → "abcd-efgh-jkmn". Typing it with or without the dashes
// works (see auth.NormalizeTempPassword).
func FormatTempPassword(p string) string {
	var b strings.Builder
	for i, r := range p {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return b.String()
}
