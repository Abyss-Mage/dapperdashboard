package email

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

// An email reader must find the two bodies and the exact file, or the customer sees no PDF.
func TestMessageWithAttachment(t *testing.T) {
	s := &SMTPSender{fromAddress: "shop@example.test", fromName: "Shop"}
	file := bytes.Repeat([]byte("%PDF synthetic \xff\x00"), 40)
	raw := s.message("customer@example.test", "Invoice", "<id@example.test>", "=_b", "<p>Hi</p>", "Hi", &Attachment{Name: "Invoice-1.pdf", ContentType: "application/pdf", Data: file})

	m, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	media, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || media != "multipart/mixed" {
		t.Fatalf("top level is %q: %v", media, err)
	}
	mixed := multipart.NewReader(m.Body, params["boundary"])

	first, err := mixed.NextRawPart()
	if err != nil {
		t.Fatal(err)
	}
	media, inner, err := mime.ParseMediaType(first.Header.Get("Content-Type"))
	if err != nil || media != "multipart/alternative" {
		t.Fatalf("first part is %q: %v", media, err)
	}
	bodies := multipart.NewReader(first, inner["boundary"])
	for _, want := range []string{"text/plain", "text/html"} {
		part, err := bodies.NextRawPart()
		if err != nil || !strings.HasPrefix(part.Header.Get("Content-Type"), want) {
			t.Fatalf("expected %s body: %v", want, err)
		}
	}

	second, err := mixed.NextRawPart()
	if err != nil {
		t.Fatal(err)
	}
	disposition, dparams, err := mime.ParseMediaType(second.Header.Get("Content-Disposition"))
	if err != nil || disposition != "attachment" || dparams["filename"] != "Invoice-1.pdf" || !strings.HasPrefix(second.Header.Get("Content-Type"), "application/pdf") {
		t.Fatalf("attachment headers wrong: %v %v", second.Header, err)
	}
	encoded, err := io.ReadAll(second)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(encoded), "\r\n"), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("base64 line of %d characters", len(line))
		}
	}
	got, err := base64.StdEncoding.DecodeString(strings.NewReplacer("\r", "", "\n", "").Replace(string(encoded)))
	if err != nil || !bytes.Equal(got, file) {
		t.Fatalf("attached file changed: %v", err)
	}
	if _, err = mixed.NextRawPart(); err != io.EOF {
		t.Fatalf("expected the message to end after the file: %v", err)
	}
}

func TestMessageWithoutAttachment(t *testing.T) {
	s := &SMTPSender{fromAddress: "shop@example.test", fromName: "Shop"}
	m, err := mail.ReadMessage(strings.NewReader(s.message("customer@example.test", "Hello", "<id@example.test>", "=_b", "", "Hi", nil)))
	if err != nil {
		t.Fatal(err)
	}
	if media, _, err := mime.ParseMediaType(m.Header.Get("Content-Type")); err != nil || media != "multipart/alternative" {
		t.Fatalf("plain email is %q: %v", media, err)
	}
}
