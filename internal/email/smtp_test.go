package email

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMIME(t *testing.T) {
	from, _ := mail.ParseAddress("All-in-one <aio@example.com>")
	to, _ := mail.ParseAddress("budi@example.com")
	raw, err := buildMIME(from, to, Message{
		Subject: "Kata sandi sementara kamu\r\nBcc: evil@example.com",
		Text:    "Hai budi — ini kata sandi sementara.",
		HTML:    "<p>Hai budi</p>",
	})
	require.NoError(t, err)

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	require.NoError(t, err)
	assert.Empty(t, msg.Header.Get("Bcc"), "a newline in the subject must not add headers")
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(subject, "Kata sandi sementara kamu"))
	assert.Equal(t, "budi@example.com", strings.Trim(msg.Header.Get("To"), "<>"))

	_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	require.NoError(t, err)
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var bodies []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		b, err := io.ReadAll(quotedprintable.NewReader(p))
		require.NoError(t, err)
		bodies = append(bodies, string(b))
	}
	assert.Equal(t, []string{"Hai budi — ini kata sandi sementara.", "<p>Hai budi</p>"}, bodies)
}
