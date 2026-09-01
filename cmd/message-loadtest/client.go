package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const maxResponseSize = 1 << 20

type loadClient struct {
	baseURL      string
	user         loadUser
	http         *http.Client
	connection   *websocket.Conn
	stats        *statistics
	outbound     chan outboundMessage
	sendStopped  chan struct{}
	done         chan struct{}
	stopSendOnce sync.Once
	closeOnce    sync.Once
	refreshMu    sync.Mutex
	conversation atomic.Uint64
}

type outboundMessage struct {
	ClientMessageID string `json:"client_message_id"`
	ConversationID  uint   `json:"conversation_id"`
	Text            string `json:"text"`
}

type apiEnvelope[T any] struct {
	Success bool      `json:"success"`
	Data    T         `json:"data"`
	Error   *apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ticketResponse struct {
	Ticket string `json:"ticket"`
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type httpResponseError struct {
	Status  int
	Code    string
	Message string
}

func (e *httpResponseError) Error() string {
	return fmt.Sprintf("HTTP %d %s: %s", e.Status, e.Code, e.Message)
}

type conversationResponse struct {
	LastMessageSeq uint64 `json:"last_message_seq"`
}

type listMessagesResponse struct {
	Items        []messageCreated `json:"items"`
	NextAfterSeq *uint64          `json:"next_after_seq"`
	HasMore      bool             `json:"has_more"`
}

type incomingEvent struct {
	Type  string          `json:"type"`
	Data  json.RawMessage `json:"data"`
	Error *apiError       `json:"error"`
}

type messageAccepted struct {
	ClientMessageID string `json:"client_message_id"`
	ConversationID  uint   `json:"conversation_id"`
}

type messageCreated struct {
	ID              uint   `json:"id"`
	ClientMessageID string `json:"client_message_id"`
	ConversationID  uint   `json:"conversation_id"`
	Seq             uint64 `json:"seq"`
	SenderID        uint   `json:"sender_id"`
}

type messageRejected struct {
	ClientMessageID string `json:"client_message_id"`
	ConversationID  uint   `json:"conversation_id"`
	SenderID        uint   `json:"sender_id"`
	Code            string `json:"code"`
}

func newLoadClient(
	ctx context.Context,
	baseURL string,
	user loadUser,
	timeout time.Duration,
	queueSize int,
	stats *statistics,
) (*loadClient, error) {
	client := &loadClient{
		baseURL:     baseURL,
		user:        user,
		http:        &http.Client{Timeout: timeout},
		stats:       stats,
		outbound:    make(chan outboundMessage, queueSize),
		sendStopped: make(chan struct{}),
		done:        make(chan struct{}),
	}
	for _, conversationID := range user.ConversationIDs {
		conversation, err := requestWithRefresh[conversationResponse](
			ctx,
			client,
			http.MethodGet,
			baseURL+"/api/v1/conversations/"+strconv.FormatUint(uint64(conversationID), 10),
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("load conversation %d: %w", conversationID, err)
		}
		stats.setBaseline(user.UserID, conversationID, conversation.LastMessageSeq)
	}

	ticket, err := requestTicket(ctx, client)
	if err != nil {
		return nil, err
	}
	wsURL, err := webSocketURL(baseURL, ticket)
	if err != nil {
		return nil, err
	}
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = timeout
	connection, response, err := dialer.DialContext(ctx, wsURL, nil)
	if response != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}
	client.connection = connection

	authenticated := make(chan error, 1)
	go client.readLoop(authenticated)
	select {
	case err := <-authenticated:
		if err != nil {
			client.close()
			return nil, err
		}
	case <-time.After(timeout):
		client.close()
		return nil, errors.New("wait auth.ok timeout")
	case <-ctx.Done():
		client.close()
		return nil, ctx.Err()
	}
	go client.writeLoop()
	return client, nil
}

func (c *loadClient) enqueueMessage(prefix string) bool {
	conversationIndex := c.conversation.Add(1) - 1
	conversationID := c.user.ConversationIDs[conversationIndex%uint64(len(c.user.ConversationIDs))]
	clientMessageID := uuid.NewString()
	message := outboundMessage{
		ClientMessageID: clientMessageID,
		ConversationID:  conversationID,
		Text:            prefix + " " + clientMessageID,
	}
	select {
	case <-c.done:
		return false
	case <-c.sendStopped:
		return false
	case c.outbound <- message:
		return true
	default:
		return false
	}
}

func (c *loadClient) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case <-c.sendStopped:
			return
		default:
		}
		select {
		case <-c.done:
			return
		case <-c.sendStopped:
			return
		case message := <-c.outbound:
			sentAt := time.Now()
			c.stats.recordSent(
				message.ClientMessageID,
				c.user.UserID,
				message.ConversationID,
				sentAt,
			)
			payload := struct {
				Type string          `json:"type"`
				Data outboundMessage `json:"data"`
			}{Type: "message.send", Data: message}
			_ = c.connection.SetWriteDeadline(sentAt.Add(10 * time.Second))
			if err := c.connection.WriteJSON(payload); err != nil {
				c.stats.recordWriteError(message.ClientMessageID)
				c.close()
				return
			}
		}
	}
}

func (c *loadClient) readLoop(authenticated chan<- error) {
	authenticatedResult := false
	defer func() {
		if !authenticatedResult {
			authenticated <- errors.New("websocket closed before auth.ok")
		}
		c.close()
	}()

	for {
		var event incomingEvent
		if err := c.connection.ReadJSON(&event); err != nil {
			if authenticatedResult {
				if closeError, ok := err.(*websocket.CloseError); ok {
					c.stats.recordWebSocketClose(closeError.Code)
				} else {
					c.stats.recordWebSocketError("READ_FAILED")
				}
			}
			return
		}
		now := time.Now()
		switch event.Type {
		case "auth.ok":
			if !authenticatedResult {
				authenticatedResult = true
				authenticated <- nil
			}
		case "message.accepted":
			var message messageAccepted
			if json.Unmarshal(event.Data, &message) != nil {
				c.stats.recordProtocolError()
				continue
			}
			c.stats.recordAccepted(message, now)
		case "message.created":
			var message messageCreated
			if json.Unmarshal(event.Data, &message) != nil {
				c.stats.recordProtocolError()
				continue
			}
			c.stats.recordCreated(c.user.UserID, message, now, true)
		case "message.rejected":
			var message messageRejected
			if json.Unmarshal(event.Data, &message) != nil {
				c.stats.recordProtocolError()
				continue
			}
			c.stats.recordRejected(message)
		case "error":
			code := "UNKNOWN"
			if event.Error != nil && event.Error.Code != "" {
				code = event.Error.Code
			}
			c.stats.recordWebSocketError(code)
		}
	}
}

func (c *loadClient) syncMessages(ctx context.Context) error {
	for _, conversationID := range c.user.ConversationIDs {
		afterSeq := c.stats.contiguousSeq(c.user.UserID, conversationID)
		for {
			path := fmt.Sprintf(
				"%s/api/v1/conversations/%d/messages?after_seq=%d&limit=100",
				c.baseURL,
				conversationID,
				afterSeq,
			)
			page, err := requestWithRefresh[listMessagesResponse](
				ctx,
				c,
				http.MethodGet,
				path,
				nil,
			)
			if err != nil {
				return fmt.Errorf("sync user %d conversation %d: %w", c.user.UserID, conversationID, err)
			}
			for _, message := range page.Items {
				c.stats.recordCreated(c.user.UserID, message, time.Now(), false)
				afterSeq = max(afterSeq, message.Seq)
			}
			if !page.HasMore {
				break
			}
			if page.NextAfterSeq == nil || *page.NextAfterSeq <= afterSeq && len(page.Items) == 0 {
				return errors.New("invalid next_after_seq")
			}
			afterSeq = *page.NextAfterSeq
		}
	}
	return nil
}

func (c *loadClient) close() {
	c.closeOnce.Do(func() {
		c.stopSending()
		close(c.done)
		if c.connection != nil {
			_ = c.connection.Close()
		}
	})
}

func (c *loadClient) stopSending() {
	c.stopSendOnce.Do(func() { close(c.sendStopped) })
}

func requestTicket(ctx context.Context, client *loadClient) (string, error) {
	ticket, err := requestWithRefresh[ticketResponse](
		ctx,
		client,
		http.MethodPost,
		client.baseURL+"/api/v1/ws/tickets",
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("request ticket: %w", err)
	}
	if ticket.Ticket == "" {
		return "", errors.New("ticket is empty")
	}
	return ticket.Ticket, nil
}

func requestWithRefresh[T any](
	ctx context.Context,
	client *loadClient,
	method string,
	requestURL string,
	body []byte,
) (T, error) {
	failedAccessToken := client.user.AccessToken
	result, err := requestJSON[T](
		ctx,
		client.http,
		method,
		requestURL,
		failedAccessToken,
		body,
	)
	var responseError *httpResponseError
	if !errors.As(err, &responseError) ||
		responseError.Code != "UNAUTHENTICATED" ||
		client.user.RefreshToken == "" {
		return result, err
	}
	if err := client.refreshTokens(ctx, failedAccessToken); err != nil {
		return result, err
	}
	return requestJSON[T](
		ctx,
		client.http,
		method,
		requestURL,
		client.user.AccessToken,
		body,
	)
}

func (c *loadClient) refreshTokens(ctx context.Context, failedAccessToken string) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	if c.user.AccessToken != failedAccessToken {
		return nil
	}
	payload, err := json.Marshal(struct {
		RefreshToken string `json:"refresh_token"`
	}{RefreshToken: c.user.RefreshToken})
	if err != nil {
		return err
	}
	refreshed, err := requestJSON[refreshResponse](
		ctx,
		c.http,
		http.MethodPost,
		c.baseURL+"/api/v1/auth/refresh",
		"",
		payload,
	)
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
		return errors.New("refresh response is invalid")
	}
	c.user.AccessToken = refreshed.AccessToken
	c.user.RefreshToken = refreshed.RefreshToken
	return nil
}

func getJSON[T any](
	ctx context.Context,
	client *http.Client,
	requestURL string,
	accessToken string,
) (T, error) {
	return requestJSON[T](ctx, client, http.MethodGet, requestURL, accessToken, nil)
}

func requestJSON[T any](
	ctx context.Context,
	client *http.Client,
	method string,
	requestURL string,
	accessToken string,
	body []byte,
) (T, error) {
	var result T
	request, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize))
	if err != nil {
		return result, err
	}
	var envelope apiEnvelope[T]
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return result, fmt.Errorf("decode HTTP %d response: %w", response.StatusCode, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !envelope.Success {
		if envelope.Error != nil {
			return result, &httpResponseError{
				Status:  response.StatusCode,
				Code:    envelope.Error.Code,
				Message: envelope.Error.Message,
			}
		}
		return result, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return envelope.Data, nil
}

func webSocketURL(baseURL string, ticket string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported base URL scheme %q", parsed.Scheme)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/v1/ws"
	query := parsed.Query()
	query.Set("ticket", ticket)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
