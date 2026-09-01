package websocket

import "encoding/json"

const (
	EventAuthOK           = "auth.ok"
	ErrorInvalidEvent     = "INVALID_EVENT"
	ErrorEventUnsupported = "EVENT_UNSUPPORTED"
)

type IncomingEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Event struct {
	Type  string      `json:"type"`
	Data  any         `json:"data,omitempty"`
	Error *EventError `json:"error,omitempty"`
}

type EventError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewErrorEvent(code, message string) Event {
	return Event{Type: "error", Error: &EventError{Code: code, Message: message}}
}

func NewErrorEventWithData(code, message string, data any) Event {
	return Event{Type: "error", Data: data, Error: &EventError{Code: code, Message: message}}
}
