// Package email sends transactional messages through Resend.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const resendEmailsURL = "https://api.resend.com/emails"

// ResendSender is a small HTTP client for Resend's email API.
type ResendSender struct {
	apiKey string
	from   string
	client *http.Client
	url    string
}

// NewResendSender creates a Resend sender with a bounded request timeout.
func NewResendSender(apiKey, from string, timeout time.Duration) *ResendSender {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &ResendSender{
		apiKey: strings.TrimSpace(apiKey),
		from:   strings.TrimSpace(from),
		client: &http.Client{Timeout: timeout},
		url:    resendEmailsURL,
	}
}

// Send sends both HTML and plain-text versions of a transactional email.
func (s *ResendSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	payload := struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HTML    string   `json:"html"`
		Text    string   `json:"text"`
	}{From: s.from, To: []string{to}, Subject: subject, HTML: htmlBody, Text: textBody}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode email request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create email request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "call-analyse-backend/1.0")
	response, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send email request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
	return fmt.Errorf("resend returned status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
}
