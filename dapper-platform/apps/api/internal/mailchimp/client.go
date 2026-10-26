package mailchimp

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrInvalidConfig  = errors.New("mailchimp configuration invalid")
	ErrInvalidEmail   = errors.New("invalid email address")
	ErrAPIError       = errors.New("mailchimp api error")
	ErrRateLimited    = errors.New("mailchimp rate limited")
	ErrMemberNotFound = errors.New("mailchimp member not found")
)

type Config struct {
	APIKey        string
	ServerPrefix  string
	DefaultListID string
	Timeout       time.Duration
}

type Subscriber struct {
	Email       string         `json:"email"`
	Status      string         `json:"status"` // "subscribed", "unsubscribed", "pending"
	FirstName   string         `json:"first_name,omitempty"`
	LastName    string         `json:"last_name,omitempty"`
	Phone       string         `json:"phone,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	MergeFields map[string]any `json:"merge_fields,omitempty"`
}

type MemberResponse struct {
	ID           string         `json:"id"`
	EmailAddress string         `json:"email_address"`
	Status       string         `json:"status"`
	MergeFields  map[string]any `json:"merge_fields"`
	Tags         []TagItem      `json:"tags"`
	LastChanged  string         `json:"last_changed"`
}

type TagItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type apiErrorResponse struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	apiKey        string
	serverPrefix  string
	defaultListID string
	httpClient    HTTPClient
}

func NewClient(cfg Config, httpClient HTTPClient) (*Client, error) {
	key := strings.TrimSpace(cfg.APIKey)
	if key == "" {
		return nil, ErrInvalidConfig
	}
	prefix := strings.TrimSpace(cfg.ServerPrefix)
	if prefix == "" {
		parts := strings.Split(key, "-")
		if len(parts) == 2 && len(parts[1]) > 0 {
			prefix = parts[1]
		} else {
			return nil, fmt.Errorf("%w: missing server prefix in api key", ErrInvalidConfig)
		}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{
		apiKey:        key,
		serverPrefix:  prefix,
		defaultListID: strings.TrimSpace(cfg.DefaultListID),
		httpClient:    httpClient,
	}, nil
}

func SubscriberHash(email string) string {
	hasher := md5.New()
	hasher.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (c *Client) endpoint(path string) string {
	return fmt.Sprintf("https://%s.api.mailchimp.com/3.0%s", c.serverPrefix, path)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path), bodyReader)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("anystring", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DapperDetailing/1.0")
	return req, nil
}

func (c *Client) Ping(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return c.parseError(resp)
}

func (c *Client) AddOrUpdateMember(ctx context.Context, listID string, sub Subscriber) (*MemberResponse, error) {
	list := strings.TrimSpace(listID)
	if list == "" {
		list = c.defaultListID
	}
	if list == "" {
		return nil, fmt.Errorf("%w: list id required", ErrInvalidConfig)
	}
	sub.Email = strings.TrimSpace(sub.Email)
	if _, err := mail.ParseAddress(sub.Email); err != nil {
		return nil, ErrInvalidEmail
	}
	status := strings.TrimSpace(sub.Status)
	if status == "" {
		status = "subscribed"
	}
	mergeFields := make(map[string]any)
	for k, v := range sub.MergeFields {
		mergeFields[k] = v
	}
	if sub.FirstName != "" {
		mergeFields["FNAME"] = sub.FirstName
	}
	if sub.LastName != "" {
		mergeFields["LNAME"] = sub.LastName
	}
	if sub.Phone != "" {
		mergeFields["PHONE"] = sub.Phone
	}

	payload := map[string]any{
		"email_address": sub.Email,
		"status_if_new": status,
		"status":        status,
		"merge_fields":  mergeFields,
	}
	if len(sub.Tags) > 0 {
		payload["tags"] = sub.Tags
	}

	hash := SubscriberHash(sub.Email)
	path := fmt.Sprintf("/lists/%s/members/%s", list, hash)
	req, err := c.newRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var member MemberResponse
		if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
			return nil, err
		}
		return &member, nil
	}
	return nil, c.parseError(resp)
}

func (c *Client) UnsubscribeMember(ctx context.Context, listID, email string) error {
	list := strings.TrimSpace(listID)
	if list == "" {
		list = c.defaultListID
	}
	if list == "" {
		return fmt.Errorf("%w: list id required", ErrInvalidConfig)
	}
	email = strings.TrimSpace(email)
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}
	hash := SubscriberHash(email)
	path := fmt.Sprintf("/lists/%s/members/%s", list, hash)
	payload := map[string]any{"status": "unsubscribed"}
	req, err := c.newRequest(ctx, http.MethodPatch, path, payload)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return c.parseError(resp)
}

func (c *Client) GetMember(ctx context.Context, listID, email string) (*MemberResponse, error) {
	list := strings.TrimSpace(listID)
	if list == "" {
		list = c.defaultListID
	}
	if list == "" {
		return nil, fmt.Errorf("%w: list id required", ErrInvalidConfig)
	}
	email = strings.TrimSpace(email)
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}
	hash := SubscriberHash(email)
	path := fmt.Sprintf("/lists/%s/members/%s", list, hash)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var member MemberResponse
		if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
			return nil, err
		}
		return &member, nil
	}
	return nil, c.parseError(resp)
}

func (c *Client) parseError(resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		return ErrRateLimited
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrMemberNotFound
	}
	var apiErr apiErrorResponse
	bodyBytes, err := io.ReadAll(resp.Body)
	if err == nil && len(bodyBytes) > 0 {
		_ = json.Unmarshal(bodyBytes, &apiErr)
	}
	if apiErr.Title != "" || apiErr.Detail != "" {
		return fmt.Errorf("%w: %s (detail: %s, status: %d)", ErrAPIError, apiErr.Title, apiErr.Detail, resp.StatusCode)
	}
	return fmt.Errorf("%w: http status %d", ErrAPIError, resp.StatusCode)
}
