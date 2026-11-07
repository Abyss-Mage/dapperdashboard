package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

var (
	ErrSMTPConfig   = errors.New("smtp configuration invalid")
	ErrSMTPSend     = errors.New("failed to send transactional email")
	ErrInvalidEmail = errors.New("invalid email address")
)

type Config struct {
	Host               string
	Port               int
	Username           string
	Password           string
	FromAddress        string
	FromName           string
	InsecureSkipVerify bool
}

type SMTPSender struct {
	tlsRoots           *x509.CertPool
	host               string
	port               int
	username           string
	password           string
	fromAddress        string
	fromName           string
	insecureSkipVerify bool
}

func NewSMTPSender(cfg Config) (*SMTPSender, error) {
	host := strings.TrimSpace(cfg.Host)
	from := strings.TrimSpace(cfg.FromAddress)
	if cfg.InsecureSkipVerify || host == "" || !validMailbox(from) || strings.ContainsAny(cfg.FromName, "\r\n") {
		return nil, ErrSMTPConfig
	}
	port := cfg.Port
	if port <= 0 {
		port = 587
	}
	fromName := strings.TrimSpace(cfg.FromName)
	if fromName == "" {
		fromName = "Dapper Detailing"
	}
	return &SMTPSender{
		host:               host,
		port:               port,
		username:           strings.TrimSpace(cfg.Username),
		password:           cfg.Password,
		fromAddress:        from,
		fromName:           fromName,
		insecureSkipVerify: cfg.InsecureSkipVerify,
	}, nil
}

// Attachment is a file sent along with an email, such as an invoice PDF.
type Attachment struct {
	Name        string
	ContentType string
	Data        []byte
}

func (s *SMTPSender) SendEmail(ctx context.Context, recipient, subject, bodyHTML, bodyText string) (string, error) {
	return s.SendEmailWithAttachment(ctx, recipient, subject, bodyHTML, bodyText, nil)
}

// SendEmailWithAttachment sends the email with a file attached; a nil attachment sends it plain.
func (s *SMTPSender) SendEmailWithAttachment(ctx context.Context, recipient, subject, bodyHTML, bodyText string, att *Attachment) (string, error) {
	if att != nil && (att.Name == "" || strings.ContainsAny(att.Name, "\r\n\"") || strings.ContainsAny(att.ContentType, "\r\n")) {
		return "", ErrInvalidEmail
	}
	recipient = strings.TrimSpace(recipient)
	if !validMailbox(recipient) || strings.ContainsAny(subject, "\r\n") {
		return "", ErrInvalidEmail
	}

	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	messageID := fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), hex.EncodeToString(nonce), s.host)
	providerRef := fmt.Sprintf("smtp-%s", hex.EncodeToString(nonce))

	boundary := fmt.Sprintf("=_dapper_%d_%s", time.Now().UnixNano(), hex.EncodeToString(nonce[:4]))

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", (&mail.Address{Name: s.fromName, Address: s.fromAddress}).String()))
	msg.WriteString(fmt.Sprintf("To: <%s>\r\n", recipient))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", subject)))
	msg.WriteString(fmt.Sprintf("Message-ID: %s\r\n", messageID))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")
	if att != nil {
		// The text and HTML bodies go in their own part, next to the file.
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s_mixed\"\r\n\r\n", boundary))
		msg.WriteString(fmt.Sprintf("--%s_mixed\r\n", boundary))
	}
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	msg.WriteString("\r\n")

	// Text Part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	msg.WriteString(encodeBody(bodyText))
	msg.WriteString("\r\n\r\n")

	// HTML Part
	if bodyHTML != "" {
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		msg.WriteString(encodeBody(bodyHTML))
		msg.WriteString("\r\n\r\n")
	}

	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	if att != nil {
		contentType := att.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		msg.WriteString(fmt.Sprintf("\r\n--%s_mixed\r\n", boundary))
		msg.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", contentType, att.Name))
		msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", att.Name))
		msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		enc := base64.StdEncoding.EncodeToString(att.Data)
		for len(enc) > 76 {
			msg.WriteString(enc[:76] + "\r\n")
			enc = enc[76:]
		}
		msg.WriteString(enc + "\r\n")
		msg.WriteString(fmt.Sprintf("--%s_mixed--\r\n", boundary))
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("%w: connection failed: %v", ErrSMTPSend, err)
	}
	defer conn.Close()
	deadline := time.Now().Add(30 * time.Second)
	if requested, ok := ctx.Deadline(); ok && requested.Before(deadline) {
		deadline = requested
	}
	if err = conn.SetDeadline(deadline); err != nil {
		return "", ErrSMTPSend
	}
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancel()

	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return "", fmt.Errorf("%w: smtp client failed: %v", ErrSMTPSend, err)
	}
	defer c.Close()

	tlsConfig := &tls.Config{
		ServerName:         s.host,
		RootCAs:            s.tlsRoots,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: s.insecureSkipVerify,
	}

	if ok, _ := c.Extension("STARTTLS"); !ok {
		return "", fmt.Errorf("%w: STARTTLS required", ErrSMTPSend)
	}
	{
		if err = c.StartTLS(tlsConfig); err != nil {
			return "", fmt.Errorf("%w: starttls failed: %v", ErrSMTPSend, err)
		}
	}

	if s.username != "" {
		// Use what the server offers: Microsoft 365 and Outlook accept only LOGIN (and XOAUTH2), most others accept PLAIN.
		auth := smtp.Auth(smtp.PlainAuth("", s.username, s.password, s.host))
		if offered, mechs := c.Extension("AUTH"); offered && !strings.Contains(strings.ToUpper(mechs), "PLAIN") && strings.Contains(strings.ToUpper(mechs), "LOGIN") {
			auth = loginAuth{s.username, s.password}
		}
		if err = c.Auth(auth); err != nil {
			return "", fmt.Errorf("%w: authentication failed: %v", ErrSMTPSend, err)
		}
	}

	if err = c.Mail(s.fromAddress); err != nil {
		return "", fmt.Errorf("%w: mail from failed: %v", ErrSMTPSend, err)
	}
	if err = c.Rcpt(recipient); err != nil {
		return "", fmt.Errorf("%w: rcpt to failed: %v", ErrSMTPSend, err)
	}

	w, err := c.Data()
	if err != nil {
		return "", fmt.Errorf("%w: data command failed: %v", ErrSMTPSend, err)
	}
	if _, err = w.Write([]byte(msg.String())); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("%w: data write failed: %v", ErrSMTPSend, err)
	}
	if err = w.Close(); err != nil {
		return "", fmt.Errorf("%w: data close failed: %v", ErrSMTPSend, err)
	}

	return providerRef, nil
}

func validMailbox(value string) bool {
	if strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value && parsed.Name == ""
}
func encodeBody(value string) string {
	var out strings.Builder
	writer := quotedprintable.NewWriter(&out)
	_, _ = writer.Write([]byte(value))
	_ = writer.Close()
	return out.String()
}

// loginAuth is the "LOGIN" sign-in some providers (Microsoft 365, Outlook) require instead of PLAIN. It only ever runs over an encrypted connection.
type loginAuth struct{ username, password string }

func (a loginAuth) Start(si *smtp.ServerInfo) (string, []byte, error) {
	if !si.TLS {
		return "", nil, errors.New("login needs an encrypted connection")
	}
	return "LOGIN", nil, nil
}

func (a loginAuth) Next(prompt []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(prompt))) {
	case "username:":
		return []byte(a.username), nil
	case "password:":
		return []byte(a.password), nil
	}
	return nil, errors.New("unexpected sign-in prompt from the mail server")
}
