package v1

import (
	"mmchat/internal/api"
	"mmchat/internal/dto"
	"mmchat/internal/middleware"
	"mmchat/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WebSocketTicketHandler struct {
	ticketIssuer   service.WebSocketTicketIssuer
	authentication gin.HandlerFunc
}

func NewWebSocketTicketHandler(
	ticketIssuer service.WebSocketTicketIssuer,
	authentication gin.HandlerFunc,
) *WebSocketTicketHandler {
	return &WebSocketTicketHandler{
		ticketIssuer:   ticketIssuer,
		authentication: authentication,
	}
}

func (h *WebSocketTicketHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/tickets", h.authentication, middleware.Wrap(h.issue))
}

func (h *WebSocketTicketHandler) issue(c *gin.Context) error {
	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	ticket, err := h.ticketIssuer.Issue(c.Request.Context(), principal.UserID, principal.SessionID)
	if err != nil {
		return err
	}

	c.Header("Cache-Control", "no-store")
	api.OK(c, http.StatusCreated, dto.WebSocketTicketResponse{
		Ticket:    ticket.Value,
		ExpiresAt: ticket.ExpiresAt,
	})
	return nil
}
