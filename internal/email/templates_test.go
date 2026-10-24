package email

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name        string
		kind        Kind
		locale      string
		data        Data
		wantSubject string
		wantText    []string
	}{
		{"temp password from an app, Indonesian", KindTempPassword, "id",
			Data{AppName: "Cashflow", Username: "budi", TempPassword: "abcdefghjkmn", Minutes: 30, Host: "aio.example.com"},
			"Kata sandi sementara kamu",
			[]string{"akun yang kamu pakai di Cashflow", "    abcd-efgh-jkmn", "30 menit", "dikelola oleh All-in-one (aio.example.com)"}},
		{"temp password without an app, English", KindTempPassword, "en",
			Data{Username: "budi", TempPassword: "abcdefghjkmn", Minutes: 30, Host: "aio.example.com"},
			"Your temporary password",
			[]string{"your All-in-one account", "current password keeps working"}},
		{"unknown locale falls back to English", KindVerifyEmail, "fr",
			Data{Username: "budi", Link: "https://aio.example.com/verify-email?token=x"},
			"Confirm your email",
			[]string{"https://aio.example.com/verify-email?token=x"}},
		{"email changed notice", KindEmailChanged, "en",
			Data{Username: "budi", NewEmail: "n•••@example.com"},
			"Your email was changed",
			[]string{"n•••@example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := render(tt.kind, tt.locale, tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSubject, msg.Subject)
			for _, want := range tt.wantText {
				assert.Contains(t, msg.Text, want)
			}
			assert.Contains(t, msg.HTML, "<html>")
		})
	}
}

func TestRenderEscapesAppNameInHTML(t *testing.T) {
	msg, err := render(KindPasswordChanged, "en", Data{AppName: `<script>x</script>`, Username: "budi"})
	require.NoError(t, err)
	assert.NotContains(t, msg.HTML, "<script>")
	assert.Contains(t, msg.HTML, "&lt;script&gt;")
}

func TestMailerRecordsAndAddressesMessages(t *testing.T) {
	rec := &Recorder{}
	m := NewMailer(rec, "aio.example.com")
	require.NoError(t, m.Send(context.Background(), KindVerifyEmail, "budi@example.com", "en", Data{Username: "budi", Link: "https://x"}))
	require.Len(t, rec.Messages(), 1)
	assert.Equal(t, "budi@example.com", rec.Messages()[0].To)
	assert.Contains(t, rec.Messages()[0].Text, "aio.example.com")
}

func TestMaskAddress(t *testing.T) {
	assert.Equal(t, "n•••@example.com", MaskAddress("nadia@example.com"))
	assert.Equal(t, "•••", MaskAddress("not-an-address"))
}
