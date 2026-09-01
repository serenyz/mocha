package dto

import "time"

type WebSocketTicketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}
