package websocket

import "sync"

type Hub struct {
	mu      sync.RWMutex
	clients map[uint]*Client
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uint]*Client)}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	previous := h.clients[client.UserID()]
	h.clients[client.UserID()] = client
	h.mu.Unlock()

	if previous != nil && previous != client {
		previous.Close(CloseConnectionReplaced, "connection replaced")
	}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	if h.clients[client.UserID()] == client {
		delete(h.clients, client.UserID())
	}
	h.mu.Unlock()
}

func (h *Hub) SendToUser(userID uint, event Event) bool {
	h.mu.RLock()
	client := h.clients[userID]
	h.mu.RUnlock()
	return client != nil && client.Send(event)
}

func (h *Hub) sendPayloadToUser(userID uint, payload []byte) bool {
	h.mu.RLock()
	client := h.clients[userID]
	h.mu.RUnlock()
	return client != nil && client.sendPayload(payload)
}

func (h *Hub) DisconnectUser(userID uint) bool {
	h.mu.RLock()
	client := h.clients[userID]
	h.mu.RUnlock()
	if client == nil {
		return false
	}
	client.Close(CloseNormal, "connection closed")
	return true
}
