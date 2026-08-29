package dto

import "time"

type UpdateMyProfileRequest struct {
	Nickname  *string `json:"nickname"`
	Gender    *uint8  `json:"gender"`
	Signature *string `json:"signature"`
	Birthday  *string `json:"birthday"`
	Country   *string `json:"country"`
	Province  *string `json:"province"`
}

type MyProfileResponse struct {
	Phone     string    `json:"phone"`
	Email     *string   `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	PublicProfile
}

type SearchUsersRequest struct {
	Phone    *string `form:"phone"`
	Nickname *string `form:"nickname"`
	Country  *string `form:"country"`
	Province *string `form:"province"`
	Age      *int    `form:"age" binding:"omitempty,min=0,max=150"`
	Gender   *uint8  `form:"gender" binding:"omitempty,max=2"`
	Cursor   *uint   `form:"cursor" binding:"omitempty,min=1"`
	Limit    *int    `form:"limit" binding:"omitempty,min=1,max=50"`
}

type SearchUserItem PublicProfile

type PublicProfile struct {
	ID           uint      `json:"id"`
	Nickname     string    `json:"nickname"`
	AvatarURL    string    `json:"avatar_url"`
	URLExpiredAt time.Time `json:"url_expired_at"`
	Gender       uint8     `json:"gender"`
	Birthday     string    `json:"birthday"`
	Country      string    `json:"country"`
	Province     string    `json:"province"`
	Signature    string    `json:"signature"`
}

type UpdateMyAvatarRequest struct {
	MediaID uint `json:"media_id" binding:"required"`
}

type UpdateMyAvatarResponse struct {
	MediaID      uint      `json:"media_id"`
	AvatarURL    string    `json:"avatar_url"`
	URLExpiredAt time.Time `json:"url_expired_at"`
}
