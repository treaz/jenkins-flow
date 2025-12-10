// Package notifier provides lightweight notification support for workflow completion.
// It supports macOS desktop notifications and optional Slack integration.
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// SlackConfig holds configuration for Slack notifications.
type SlackConfig struct {
	WebhookURL string // Slack incoming webhook URL
	Channel    string // Optional: override default channel
	Username   string // Optional: bot username
}

// Config holds the notifier configuration.
type Config struct {
	Slack *SlackConfig // nil if Slack is not configured
}

// Notifier handles sending notifications to various channels.
type Notifier struct {
	config Config
}

// New creates a new Notifier with the given configuration.
func New(cfg Config) *Notifier {
	return &Notifier{config: cfg}
}

// NewFromEnv creates a new Notifier with configuration from environment variables.
// Environment variables:
//   - SLACK_WEBHOOK_URL: Slack incoming webhook URL (enables Slack notifications)
//   - SLACK_CHANNEL: Optional channel override
//   - SLACK_USERNAME: Optional username override
func NewFromEnv() *Notifier {
	var slackCfg *SlackConfig

	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	if webhookURL != "" {
		slackCfg = &SlackConfig{
			WebhookURL: webhookURL,
			Channel:    os.Getenv("SLACK_CHANNEL"),
			Username:   os.Getenv("SLACK_USERNAME"),
		}
	}

	return New(Config{Slack: slackCfg})
}

// Notify sends a notification through all configured channels.
// It sends a macOS desktop notification and optionally a Slack message.
// Errors from notification delivery are logged but not returned to avoid
// breaking the CLI flow.
func (n *Notifier) Notify(success bool, title, message string) {
	// Always send macOS notification
	sendMacOSNotification(title, message)

	// Send Slack notification if configured
	if n.config.Slack != nil {
		sendSlackNotification(n.config.Slack, success, title, message)
	}
}

// sendMacOSNotification sends a desktop notification using osascript.
// Errors are silently ignored to prevent notification failures from breaking the CLI.
func sendMacOSNotification(title, message string) {
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, escapeAppleScript(message), escapeAppleScript(title))
	cmd := exec.Command("osascript", "-e", script)
	_ = cmd.Run() // Ignore errors - don't let notification failures break the CLI
}

// escapeAppleScript escapes special characters for AppleScript strings.
func escapeAppleScript(s string) string {
	// Escape backslashes first, then double quotes
	result := ""
	for _, r := range s {
		switch r {
		case '\\':
			result += "\\\\"
		case '"':
			result += "\\\""
		default:
			result += string(r)
		}
	}
	return result
}

// slackMessage represents the Slack webhook message payload.
type slackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

// slackAttachment represents a Slack message attachment.
type slackAttachment struct {
	Color string `json:"color"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// sendSlackNotification sends a notification to Slack via webhook.
// Errors are silently ignored to prevent notification failures from breaking the CLI.
func sendSlackNotification(cfg *SlackConfig, success bool, title, message string) {
	color := "#36a64f" // green for success
	if !success {
		color = "#dc3545" // red for failure
	}

	msg := slackMessage{
		Channel:  cfg.Channel,
		Username: cfg.Username,
		Attachments: []slackAttachment{
			{
				Color: color,
				Title: title,
				Text:  message,
			},
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return // Silently ignore
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", cfg.WebhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return // Silently ignore
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return // Silently ignore
	}
	defer resp.Body.Close()
	// Response is intentionally not checked - we don't want to break CLI on Slack errors
}
