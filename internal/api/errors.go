package api

import (
	"net/http"
)

type AppError struct {
	status  int
	code    string
	message string
}

func New(status int, code, message string) *AppError {
	return &AppError{
		status:  status,
		code:    code,
		message: message,
	}
}

func (e *AppError) Error() string { return e.code }

func (e *AppError) Status() int { return e.status }

func (e *AppError) Code() string { return e.code }

func (e *AppError) Message() string { return e.message }

var (
	ErrRegisterCodeCooldown    = New(http.StatusTooManyRequests, "REGISTER_CODE_COOLDOWN", "验证码发送过于频繁，请稍后重试")
	ErrRegisterCodeHourlyLimit = New(http.StatusTooManyRequests, "REGISTER_CODE_HOURLY_LIMIT", "验证码发送次数过多，请稍后再试")
	ErrInvalidArgument         = New(http.StatusBadRequest, "INVALID_ARGUMENT", "请求参数错误")
	ErrInvalidPhone            = New(http.StatusBadRequest, "INVALID_PHONE", "手机号格式不正确")
	ErrWeakPassword            = New(http.StatusBadRequest, "WEAK_PASSWORD", "密码格式不符合要求")
	ErrInvalidNickname         = New(http.StatusBadRequest, "INVALID_NICKNAME", "昵称格式不符合要求")
	ErrPhoneRegistered         = New(http.StatusConflict, "PHONE_REGISTERED", "手机号已经注册")
	ErrRegisterCodeInvalid     = New(http.StatusUnprocessableEntity, "REGISTER_CODE_INVALID", "验证码错误")
	ErrRegisterCodeExpired     = New(http.StatusUnprocessableEntity, "REGISTER_CODE_EXPIRED", "验证码已过期")
	ErrInvalidCredentials      = New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "手机号或密码错误")
	ErrAccountDisabled         = New(http.StatusForbidden, "ACCOUNT_DISABLED", "账号不可用")
	ErrUnauthenticated         = New(http.StatusUnauthorized, "UNAUTHENTICATED", "请先登录")
	ErrInvalidRefreshToken     = New(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "登录状态已失效，请重新登录")
	ErrUserNotFound            = New(http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
	ErrInvalidGender           = New(http.StatusBadRequest, "INVALID_GENDER", "性别参数不正确")
	ErrInvalidSignature        = New(http.StatusBadRequest, "INVALID_SIGNATURE", "个性签名格式不正确")
	ErrInvalidBirthday         = New(http.StatusBadRequest, "INVALID_BIRTHDAY", "生日格式不正确")
	ErrInvalidCountry          = New(http.StatusBadRequest, "INVALID_COUNTRY", "国家或地区格式不正确")
	ErrInvalidProvince         = New(http.StatusBadRequest, "INVALID_PROVINCE", "省份或一级行政区格式不正确")
	ErrInvalidMediaType        = New(http.StatusBadRequest, "INVALID_MEDIA_TYPE", "媒体类型不正确")
	ErrUnsupportedMediaFormat  = New(http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_FORMAT", "不支持的媒体格式")
	ErrMediaTooLarge           = New(http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "媒体文件过大")
	ErrMediaNotFound           = New(http.StatusNotFound, "MEDIA_NOT_FOUND", "媒体不存在")
	ErrMediaUploadExpired      = New(http.StatusGone, "MEDIA_UPLOAD_EXPIRED", "上传申请已过期")
	ErrMediaUploadIncomplete   = New(http.StatusConflict, "MEDIA_UPLOAD_INCOMPLETE", "媒体文件尚未上传完成")
	ErrMediaSizeMismatch       = New(http.StatusUnprocessableEntity, "MEDIA_SIZE_MISMATCH", "媒体文件大小与申请不一致")
	ErrMediaMIMETypeMismatch   = New(http.StatusUnprocessableEntity, "MEDIA_MIME_TYPE_MISMATCH", "媒体文件格式与申请不一致")
	ErrMediaStatusConflict     = New(http.StatusConflict, "MEDIA_STATUS_CONFLICT", "媒体状态不允许执行当前操作")
)
