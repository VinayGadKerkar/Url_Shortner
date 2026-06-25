package models

import "time"

// ClickEvent is published to Kafka on every redirect.
// The analytics consumer reads these events and updates click counts asynchronously.
type ClickEvent struct {
	ShortCode  string    `json:"short_code"`
	AccessedAt time.Time `json:"accessed_at"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
}
