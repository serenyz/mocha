package dto

type SendRegisterCodeRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type SendRegisterCodeResponse struct {
	ExpireIn int `json:"expire_in" binding:"required"`
	RetryIn  int `json:"retry_in" binding:"required"`
}

type RegisterRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=64"`
	Nickname string `json:"nickname" binding:"required,min=1,max=50"`
	Code     string `json:"code" binding:"required,len=6"`
}

type LoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken      string           `json:"access_token"`
	RefreshToken     string           `json:"refresh_token"`
	TokenType        string           `json:"token_type"`
	ExpiresIn        int64            `json:"expires_in"`
	RefreshExpiresIn int64            `json:"refresh_expires_in"`
	User             LoginUserSummary `json:"user"`
}

type LoginUserSummary struct {
	UUID     string `json:"uuid"`
	Nickname string `json:"nickname"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RefreshResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
}
