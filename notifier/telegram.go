package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// TelegramNotifier sends messages to a Telegram chat.
type TelegramNotifier struct {
	apiKey     string
	userID     string
	httpClient *http.Client
}

// NewTelegramNotifier creates a new notifier for Telegram.
func NewTelegramNotifier(apiKey, userID string) *TelegramNotifier {
	return &TelegramNotifier{
		apiKey: apiKey,
		userID: userID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Notify sends the given message.
func (t *TelegramNotifier) Notify(message string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.apiKey)

	// Telegram messages have a size limit of 4096 characters.
	if len(message) > 4096 {
		message = message[:4093] + "..."
	}

	params := url.Values{}
	params.Add("chat_id", t.userID)
	params.Add("text", message)
	params.Add("parse_mode", "Markdown") // Or "HTML"

	req, err := http.NewRequest("POST", apiURL, bytes.NewBufferString(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API returned non-200 status: %d - %s", resp.StatusCode, string(body))
	}

	var tgResp struct {
		Ok bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return fmt.Errorf("failed to decode telegram response: %w", err)
	}
	if !tgResp.Ok {
		return fmt.Errorf("telegram API indicated failure")
	}

	return nil
}
