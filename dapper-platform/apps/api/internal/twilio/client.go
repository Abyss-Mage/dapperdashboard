// Package twilio sends text messages through Twilio's REST API. It only needs an Account SID, an Auth Token and either a Twilio
// phone number or a Messaging Service SID to send from.
package twilio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrConfig  = errors.New("twilio settings invalid")
	ErrPhone   = errors.New("enter the phone number with the country code, for example +17055550100")
	ErrAuth    = errors.New("Twilio did not accept the Account SID and Auth Token")
	ErrRequest = errors.New("Twilio could not send the message")
)

var (
	accountSID = regexp.MustCompile(`^AC[0-9a-fA-F]{32}$`)
	serviceSID = regexp.MustCompile(`^MG[0-9a-fA-F]{32}$`)
	e164       = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

// Config is one Twilio account. From is a Twilio phone number in +E.164 form or a Messaging Service SID (starts with MG).
type Config struct {
	AccountSID string
	AuthToken  string
	From       string
}

// ValidSID, ValidFrom and ValidPhone are used by the dashboard before anything is saved.
func ValidSID(v string) bool   { return accountSID.MatchString(v) }
func ValidFrom(v string) bool  { return e164.MatchString(v) || serviceSID.MatchString(v) }
func ValidPhone(v string) bool { return e164.MatchString(v) }

// NormalizePhone strips spaces, dashes and brackets from a typed number.
func NormalizePhone(v string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(v) {
		if (r >= '0' && r <= '9') || r == '+' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (c Config) valid() error {
	if !ValidSID(c.AccountSID) || len(c.AuthToken) < 16 || len(c.AuthToken) > 100 || strings.ContainsAny(c.AuthToken, " \r\n\t") || !ValidFrom(c.From) {
		return ErrConfig
	}
	return nil
}

// Client talks to Twilio. BaseURL and HTTP can be replaced in tests.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient() *Client {
	return &Client{BaseURL: "https://api.twilio.com", HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) do(ctx context.Context, cfg Config, method, path string, form url.Values) (int, []byte, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return 0, nil, err
	}
	req.SetBasicAuth(cfg.AccountSID, cfg.AuthToken)
	req.Header.Set("Accept", "application/json")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, ErrRequest
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, nil
}

// Verify checks that the Account SID and Auth Token work. It sends nothing and costs nothing.
func (c *Client) Verify(ctx context.Context, cfg Config) error {
	if err := cfg.valid(); err != nil {
		return err
	}
	status, _, err := c.do(ctx, cfg, http.MethodGet, "/2010-04-01/Accounts/"+cfg.AccountSID+".json", nil)
	if err != nil {
		return err
	}
	switch {
	case status == http.StatusOK:
		return nil
	case status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound:
		return ErrAuth
	}
	return ErrRequest
}

// Send texts one message and returns Twilio's message SID. to must be a +E.164 number.
func (c *Client) Send(ctx context.Context, cfg Config, to, body string) (string, error) {
	if err := cfg.valid(); err != nil {
		return "", err
	}
	to = NormalizePhone(to)
	if !ValidPhone(to) {
		return "", ErrPhone
	}
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 1600 {
		return "", ErrConfig
	}
	form := url.Values{"To": {to}, "Body": {body}}
	if serviceSID.MatchString(cfg.From) {
		form.Set("MessagingServiceSid", cfg.From)
	} else {
		form.Set("From", cfg.From)
	}
	status, data, err := c.do(ctx, cfg, http.MethodPost, "/2010-04-01/Accounts/"+cfg.AccountSID+"/Messages.json", form)
	if err != nil {
		return "", err
	}
	var reply struct {
		SID     string `json:"sid"`
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
	_ = json.Unmarshal(data, &reply)
	if status == http.StatusUnauthorized {
		return "", ErrAuth
	}
	if status < 200 || status > 299 || reply.SID == "" {
		if reply.Message != "" {
			return "", fmt.Errorf("%w: %s", ErrRequest, reply.Message)
		}
		return "", ErrRequest
	}
	return reply.SID, nil
}
