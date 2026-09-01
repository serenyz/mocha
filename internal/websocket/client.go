package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	CloseNormal             = websocket.CloseNormalClosure
	CloseConnectionReplaced = 4003
	CloseSlowClient         = 4004
)

var errInvalidEvent = errors.New("invalid websocket event")

type clientConfig struct {
	pingInterval   time.Duration
	pongTimeout    time.Duration
	writeTimeout   time.Duration
	maxMessageSize int64
	sendQueueSize  int
}

type outboundMessage struct {
	payload []byte
	written chan struct{}
}

type closeRequest struct {
	code   int
	reason string
	closed chan struct{}
}

type Client struct {
	hub        *Hub
	connection *websocket.Conn
	handler    EventHandler
	config     clientConfig
	remoteAddr string

	send          chan outboundMessage
	closeRequests chan closeRequest
	done          chan struct{}

	closeOnce sync.Once

	identity   Identity
	lastPongAt atomic.Int64
}

func newClient(
	hub *Hub,
	connection *websocket.Conn,
	handler EventHandler,
	config clientConfig,
	identity Identity,
	remoteAddr string,
) *Client {
	return &Client{
		hub:           hub,
		connection:    connection,
		handler:       handler,
		config:        config,
		identity:      identity,
		remoteAddr:    remoteAddr,
		send:          make(chan outboundMessage, config.sendQueueSize),
		closeRequests: make(chan closeRequest, 1),
		done:          make(chan struct{}),
	}
}

func (c *Client) UserID() uint { return c.identity.UserID }

func (c *Client) SessionID() string { return c.identity.SessionID }

func (c *Client) LastPongAt() time.Time {
	return time.Unix(0, c.lastPongAt.Load()).UTC()
}

func (c *Client) Serve(ctx context.Context) {
	go c.writeLoop()
	c.hub.Register(c)
	c.Send(Event{Type: EventAuthOK})
	c.readLoop(ctx)
	c.hub.Unregister(c)
	c.shutdown()
}

func (c *Client) Send(event Event) bool {
	payload, err := json.Marshal(event)
	if err != nil {
		return false
	}
	return c.sendPayload(payload)
}

func (c *Client) sendPayload(payload []byte) bool {
	return c.enqueue(outboundMessage{payload: payload})
}

func (c *Client) Close(code int, reason string) {
	c.requestClose(closeRequest{code: code, reason: reason})
}

func (c *Client) readLoop(ctx context.Context) {
	c.connection.SetReadLimit(c.config.maxMessageSize)
	now := time.Now()

	c.lastPongAt.Store(now.UnixNano())
	_ = c.connection.SetReadDeadline(time.Now().Add(c.config.pongTimeout))
	c.connection.SetPongHandler(func(string) error {
		now := time.Now()
		c.lastPongAt.Store(now.UnixNano())
		return c.connection.SetReadDeadline(now.Add(c.config.pongTimeout))
	})

	for {
		event, err := c.readEvent()
		if err != nil {
			if errors.Is(err, errInvalidEvent) {
				c.Send(NewErrorEvent(ErrorInvalidEvent, "事件格式不正确"))
				continue
			}
			return
		}
		c.handler.Handle(ctx, c, event)
	}
}

func (c *Client) readEvent() (IncomingEvent, error) {
	messageType, payload, err := c.connection.ReadMessage()
	if err != nil {
		return IncomingEvent{}, err
	}
	if messageType != websocket.TextMessage {
		return IncomingEvent{}, errInvalidEvent
	}

	var event IncomingEvent
	if json.Unmarshal(payload, &event) != nil || strings.TrimSpace(event.Type) == "" {
		return IncomingEvent{}, errInvalidEvent
	}
	return event, nil
}

func (c *Client) enqueue(message outboundMessage) bool {
	select {
	case <-c.done:
		return false
	case c.send <- message:
		return true
	default:
		c.Close(CloseSlowClient, "send queue full")
		return false
	}
}

func (c *Client) sendFinal(event Event, closeCode int, closeReason string) {
	payload, err := json.Marshal(event)
	if err != nil {
		c.Close(closeCode, closeReason)
		return
	}

	written := make(chan struct{})
	if !c.enqueue(outboundMessage{payload: payload, written: written}) {
		return
	}
	select {
	case <-written:
	case <-time.After(c.config.writeTimeout):
	}

	closed := make(chan struct{})
	if !c.requestClose(closeRequest{code: closeCode, reason: closeReason, closed: closed}) {
		return
	}
	select {
	case <-closed:
	case <-time.After(c.config.writeTimeout):
		c.shutdown()
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(c.config.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case request := <-c.closeRequests:
			c.writeClose(request)
			return
		default:
		}

		select {
		case message := <-c.send:
			_ = c.connection.SetWriteDeadline(time.Now().Add(c.config.writeTimeout))
			err := c.connection.WriteMessage(websocket.TextMessage, message.payload)
			if message.written != nil {
				close(message.written)
			}
			if err != nil {
				c.shutdown()
				return
			}
		case <-ticker.C:
			deadline := time.Now().Add(c.config.writeTimeout)
			if c.connection.WriteControl(websocket.PingMessage, nil, deadline) != nil {
				c.shutdown()
				return
			}
		case request := <-c.closeRequests:
			c.writeClose(request)
			return
		case <-c.done:
			return
		}
	}
}

func (c *Client) requestClose(request closeRequest) bool {
	select {
	case <-c.done:
		return false
	case c.closeRequests <- request:
		return true
	default:
		c.shutdown()
		return false
	}
}

func (c *Client) writeClose(request closeRequest) {
	deadline := time.Now().Add(c.config.writeTimeout)
	_ = c.connection.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(request.code, request.reason),
		deadline,
	)
	if request.closed != nil {
		close(request.closed)
	}
	c.shutdown()
}

func (c *Client) shutdown() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.connection.Close()
	})
}
