package common

import (
	"mmchat/internal/api"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var mainlandPhonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

func NormalizeMainlandPhone(raw string) (string, error) {
	phone := strings.TrimSpace(raw)
	if !mainlandPhonePattern.MatchString(phone) {
		return "", api.ErrInvalidPhone
	}

	return phone, nil
}

func ValidatePassword(password string) error {
	if !utf8.ValidString(password) {
		return api.ErrWeakPassword
	}

	if utf8.RuneCountInString(password) < 8 || len([]byte(password)) > 64 {
		return api.ErrWeakPassword
	}

	if strings.TrimSpace(password) == "" {
		return api.ErrWeakPassword
	}

	for _, r := range password {
		if unicode.IsControl(r) {
			return api.ErrWeakPassword
		}
	}

	return nil
}

func NormalizeNickname(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", api.ErrInvalidNickname
	}

	// 将等价的 Unicode 表示统一为 NFC。
	nickname := norm.NFC.String(raw)

	for _, r := range nickname {
		// Cc：换行、制表符等控制字符
		if unicode.IsControl(r) {
			return "", api.ErrInvalidNickname
		}

		// Cf：零宽字符、双向文本控制符等格式字符
		if unicode.Is(unicode.Cf, r) {
			return "", api.ErrInvalidNickname
		}
	}

	// 去除首尾空白，并压缩连续空白。
	nickname = strings.Join(strings.Fields(nickname), " ")

	length := utf8.RuneCountInString(nickname)
	if length < 1 || length > 50 {
		return "", api.ErrInvalidNickname
	}

	return nickname, nil
}

func NormalizeSignature(
	raw string,
) (string, error) {
	if !utf8.ValidString(raw) {
		return "", api.ErrInvalidSignature
	}

	signature := norm.NFC.String(raw)

	for _, r := range signature {
		if unicode.IsControl(r) {
			return "", api.ErrInvalidSignature
		}

		if unicode.Is(unicode.Cf, r) {
			return "", api.ErrInvalidSignature
		}
	}

	signature = strings.TrimSpace(signature)

	if utf8.RuneCountInString(signature) > 200 {
		return "", api.ErrInvalidSignature
	}

	return signature, nil
}

func ParseBirthday(raw string, now time.Time) (time.Time, error) {
	location := now.Location()

	birthday, err := time.ParseInLocation(
		time.DateOnly,
		strings.TrimSpace(raw),
		location,
	)
	if err != nil {
		return time.Time{}, api.ErrInvalidBirthday
	}

	today := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		0, 0, 0, 0,
		location,
	)
	if birthday.After(today) {
		return time.Time{}, api.ErrInvalidBirthday
	}

	return birthday, nil
}

func NormalizeCountry(raw string) (string, error) {
	country := strings.ToUpper(strings.TrimSpace(raw))
	if country == "" {
		return "", nil
	}

	if len(country) != 2 ||
		country[0] < 'A' || country[0] > 'Z' ||
		country[1] < 'A' || country[1] > 'Z' {
		return "", api.ErrInvalidCountry
	}

	return country, nil
}

func NormalizeProvince(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", api.ErrInvalidProvince
	}

	province := norm.NFC.String(raw)
	for _, r := range province {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", api.ErrInvalidProvince
		}
	}

	province = strings.Join(strings.Fields(province), " ")
	if utf8.RuneCountInString(province) > 100 {
		return "", api.ErrInvalidProvince
	}

	return province, nil
}
