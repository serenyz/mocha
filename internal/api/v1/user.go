package v1

import (
	"fmt"
	"mmchat/internal/api"
	"mmchat/internal/dto"
	"mmchat/internal/middleware"
	"mmchat/internal/service"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	userService    service.UserService
	authentication gin.HandlerFunc
}

func NewUserHandler(userService service.UserService, authentication gin.HandlerFunc) *UserHandler {
	return &UserHandler{userService: userService, authentication: authentication}
}

func (h *UserHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.Use(h.authentication)
	group.GET("/me", middleware.Wrap(h.getMe))
	group.PATCH("/me", middleware.Wrap(h.updateMe))
	group.PUT("/me/avatar", middleware.Wrap(h.updateMyAvatar))
	group.GET("", middleware.Wrap(h.searchUsers))
}

func (h *UserHandler) getMe(c *gin.Context) error {
	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}
	cmd := &service.GetMeCommand{UUID: principal.UserUUID}
	res := &service.GetMeRes{}
	if err := h.userService.GetMe(c.Request.Context(), cmd, res); err != nil {
		return err
	}
	api.OK[dto.MyProfileResponse](c, http.StatusOK, dto.MyProfileResponse{
		Phone: res.Phone,
		Email: res.Email,
		PublicProfile: dto.PublicProfile{
			UUID:         res.UUID,
			Nickname:     res.Nickname,
			AvatarURL:    res.AvatarURL,
			URLExpiredAt: res.URLExpiredAt,
			Gender:       res.Gender,
			Signature:    res.Signature,
			Birthday:     res.Birthday.Format(time.DateOnly),
			Country:      res.Country,
			Province:     res.Province,
		},
		CreatedAt: res.CreatedAt,
	})
	return nil
}

func (h *UserHandler) updateMe(c *gin.Context) error {
	var req dto.UpdateMyProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.UpdateMeCommand{
		UUID:      principal.UserUUID,
		Nickname:  req.Nickname,
		Gender:    req.Gender,
		Signature: req.Signature,
		Birthday:  req.Birthday,
		Country:   req.Country,
		Province:  req.Province,
	}
	res := &service.UpdateMeRes{}
	if err := h.userService.UpdateMe(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	api.OK[dto.MyProfileResponse](c, http.StatusOK, dto.MyProfileResponse{
		Phone: res.Phone,
		Email: res.Email,
		PublicProfile: dto.PublicProfile{
			UUID:         res.UUID,
			Nickname:     res.Nickname,
			AvatarURL:    res.AvatarURL,
			URLExpiredAt: res.URLExpiredAt,
			Gender:       res.Gender,
			Signature:    res.Signature,
			Birthday:     res.Birthday.Format(time.DateOnly),
			Country:      res.Country,
			Province:     res.Province,
		},
		CreatedAt: res.CreatedAt,
	})
	return nil
}

func (h *UserHandler) updateMyAvatar(c *gin.Context) error {
	var req dto.UpdateMyAvatarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	mediaUUID, err := uuid.Parse(req.MediaUUID)
	if err != nil {
		return api.ErrInvalidArgument
	}

	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.UpdateAvatarCommand{
		UserUUID:  principal.UserUUID,
		MediaUUID: mediaUUID.String(),
	}
	res := &service.UpdateAvatarRes{}

	if err := h.userService.UpdateAvatar(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	api.OK[dto.UpdateMyAvatarResponse](c, http.StatusOK, dto.UpdateMyAvatarResponse{
		MediaUUID:    res.MediaUUID,
		AvatarURL:    res.AvatarURL,
		URLExpiredAt: res.URLExpiredAt,
	})
	return nil
}

func (h *UserHandler) searchUsers(c *gin.Context) error {
	var req dto.SearchUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	cmd := &service.SearchUsersCommand{
		Phone:    req.Phone,
		Nickname: req.Nickname,
		Country:  req.Country,
		Province: req.Province,
		Gender:   req.Gender,
		Age:      req.Age,
		Cursor:   req.Cursor,
	}
	if req.Limit != nil {
		cmd.Limit = *req.Limit
	}
	res := &service.SearchUsersRes{}
	if err := h.userService.SearchUsers(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	users := make([]*dto.SearchUserItem, 0, len(res.Users))
	for _, user := range res.Users {
		users = append(users, &dto.SearchUserItem{
			UUID:         user.UUID,
			Nickname:     user.Nickname,
			AvatarURL:    user.AvatarURL,
			URLExpiredAt: user.URLExpiredAt,
			Gender:       user.Gender,
			Signature:    user.Signature,
			Birthday:     user.Birthday.Format(time.DateOnly),
			Country:      user.Country,
			Province:     user.Province,
		})
	}
	api.OKWithPage[[]*dto.SearchUserItem](
		c, http.StatusOK, users, api.CursorMata{
			NextCursor: res.NextCursor,
			HasMore:    res.HasMore,
			Limit:      res.Limit,
		})
	return nil
}
