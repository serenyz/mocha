package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestOutboundMessageJSON(t *testing.T) {
	payload, err := json.Marshal(struct {
		Type string          `json:"type"`
		Data outboundMessage `json:"data"`
	}{
		Type: "message.send",
		Data: outboundMessage{
			ClientMessageID: "message-1",
			ConversationID:  2,
			Text:            "hello",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `{"type":"message.send","data":{"client_message_id":"message-1","conversation_id":2,"text":"hello"}}`
	if string(payload) != want {
		t.Fatalf("unexpected payload: %s", payload)
	}
}

func TestSequenceStateHandlesOutOfOrderMessages(t *testing.T) {
	state := &sequenceState{
		Contiguous: 10,
		Max:        10,
		Seen:       make(map[uint64]struct{}),
	}
	if !state.add(12) || state.Contiguous != 10 || state.gaps() != 1 {
		t.Fatalf("unexpected state after seq 12: %+v", state)
	}
	if !state.add(11) || state.Contiguous != 12 || state.gaps() != 0 {
		t.Fatalf("unexpected state after seq 11: %+v", state)
	}
	if state.add(12) {
		t.Fatal("duplicate seq was accepted")
	}
}

func TestPercentile(t *testing.T) {
	values := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
	}
	if got := percentile(values, 50); got != 20*time.Millisecond {
		t.Fatalf("p50 = %s", got)
	}
	if got := percentile(values, 99); got != 40*time.Millisecond {
		t.Fatalf("p99 = %s", got)
	}
}

func TestLoadClientMessageLifecycle(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/api/v1/conversations/9":
			writeTestJSON(writer, `{"success":true,"data":{"last_message_seq":0}}`)
		case request.URL.Path == "/api/v1/ws/tickets":
			writeTestJSON(writer, `{"success":true,"data":{"ticket":"ticket-1"}}`)
		case request.URL.Path == "/api/v1/conversations/9/messages":
			writeTestJSON(writer, `{"success":true,"data":{"items":[],"has_more":false}}`)
		case request.URL.Path == "/api/v1/ws":
			connection, err := upgrader.Upgrade(writer, request, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer connection.Close()
			if err := connection.WriteJSON(map[string]any{"type": "auth.ok"}); err != nil {
				t.Error(err)
				return
			}

			var event incomingEvent
			if err := connection.ReadJSON(&event); err != nil {
				t.Error(err)
				return
			}
			var message outboundMessage
			if err := json.Unmarshal(event.Data, &message); err != nil {
				t.Error(err)
				return
			}
			accepted := map[string]any{
				"type": "message.accepted",
				"data": map[string]any{
					"client_message_id": message.ClientMessageID,
					"conversation_id":   message.ConversationID,
				},
			}
			created := map[string]any{
				"type": "message.created",
				"data": map[string]any{
					"id":                1,
					"client_message_id": message.ClientMessageID,
					"conversation_id":   message.ConversationID,
					"seq":               1,
					"sender_id":         1,
				},
			}
			if err := connection.WriteJSON(accepted); err != nil {
				t.Error(err)
				return
			}
			if err := connection.WriteJSON(created); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	users := []loadUser{{UserID: 1, AccessToken: "token", ConversationIDs: []uint{9}}}
	stats := newStatistics(users)
	client, err := newLoadClient(
		context.Background(),
		server.URL,
		users[0],
		time.Second,
		1,
		stats,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.close()

	stats.start()
	if !client.enqueueMessage("load-test") {
		t.Fatal("enqueue message failed")
	}
	stats.recordOffered()
	waitForCreated(t, stats)
	client.stopSending()
	stats.stopSending()
	if err := client.syncMessages(context.Background()); err != nil {
		t.Fatal(err)
	}
	stats.finish()
	if stats.failed() {
		t.Fatal("message lifecycle was reported as failed")
	}
}

func writeTestJSON(writer http.ResponseWriter, payload string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(payload))
}

func waitForCreated(t *testing.T, stats *statistics) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		stats.mu.Lock()
		complete := stats.accepted == 1 && stats.created == 1
		stats.mu.Unlock()
		if complete {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("message was not accepted and created")
}
