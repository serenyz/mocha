package v1

import (
	"fmt"
	"mmchat/internal/api"
	"mmchat/internal/dto"
	"mmchat/internal/middleware"
	"mmchat/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService    service.AuthService
	authentication gin.HandlerFunc
}

func NewAuthHandler(authService service.AuthService, authentication gin.HandlerFunc) *AuthHandler {
	return &AuthHandler{authService: authService, authentication: authentication}
}

func (h *AuthHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/register-code", middleware.Wrap(h.sendRegisterCode))
	group.POST("/register", middleware.Wrap(h.register))
	group.POST("/login", middleware.Wrap(h.login))
	group.POST("/refresh", middleware.Wrap(h.refresh))
	group.POST("/logout", h.authentication, middleware.Wrap(h.logout))
}

func (h *AuthHandler) sendRegisterCode(c *gin.Context) error {
	var req dto.SendRegisterCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	cmd := &service.SendCodeCommand{Phone: req.Phone}
	err := h.authService.SendRegisterCode(c.Request.Context(), cmd)
	if err != nil {
		return err
	}
	api.Success(c, http.StatusOK)
	return nil
}

func (h *AuthHandler) register(c *gin.Context) error {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	err := h.authService.Register(c.Request.Context(), &service.RegisterCommand{
		Phone:    req.Phone,
		Password: req.Password,
		Nickname: req.Nickname,
		Code:     req.Code,
	})
	if err != nil {
		return err
	}

	api.Success(c, http.StatusCreated)
	return err
}

func (h *AuthHandler) login(c *gin.Context) error {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	cmd := &service.LoginCommand{
		Phone:    req.Phone,
		Password: req.Password,
	}
	res := &service.LoginRes{}
	err := h.authService.Login(c.Request.Context(), cmd, res)
	if err != nil {
		return err
	}

	api.OK[dto.LoginResponse](c, http.StatusOK, dto.LoginResponse{
		AccessToken:      res.AccessToken,
		RefreshToken:     res.RefreshToken,
		TokenType:        "Bearer",
		ExpiresIn:        res.ExpiresIn,
		RefreshExpiresIn: res.RefreshExpiresIn,
		User: dto.LoginUserSummary{
			ID:       res.User.ID,
			Nickname: res.User.Nickname,
		},
	})
	return nil
}

func (h *AuthHandler) refresh(c *gin.Context) error {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}
	cmd := &service.RefreshCommand{RefreshToken: req.RefreshToken}
	res := &service.RefreshRes{}
	if err := h.authService.Refresh(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	api.OK[dto.RefreshResponse](c, http.StatusOK, dto.RefreshResponse{
		AccessToken:      res.AccessToken,
		RefreshToken:     res.RefreshToken,
		TokenType:        "Bearer",
		ExpiresIn:        res.ExpiresIn,
		RefreshExpiresIn: res.RefreshExpiresIn,
	})
	return nil
}

func (h *AuthHandler) logout(c *gin.Context) error {
	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}
	if err := h.authService.Logout(
		c.Request.Context(),
		principal.UserID,
		principal.SessionID,
	); err != nil {
		return err
	}
	api.Success(c, http.StatusOK)
	return nil
}
