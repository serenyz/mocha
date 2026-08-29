package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidAccessToken  = errors.New("invalid access token")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

type AccessClaims struct {
	UserID    uint
	SessionID string
}

type AccessToken struct {
	Value     string
	ExpiresIn int64
}

type RefreshToken struct {
	Value     string
	Hash      string
	UserID    uint
	SessionID string
}

type AccessTokenIssuer interface {
	Issue(claims AccessClaims) (*AccessToken, error)
}

type AccessTokenVerifier interface {
	Verify(rawToken string) (*AccessClaims, error)
}

type AccessTokenProvider interface {
	AccessTokenIssuer
	AccessTokenVerifier
}

type jwtTokenProvider struct {
	issuer string
	secret []byte
	ttl    time.Duration
}

var _ AccessTokenIssuer = (*jwtTokenProvider)(nil)
var _ AccessTokenVerifier = (*jwtTokenProvider)(nil)
var _ AccessTokenProvider = (*jwtTokenProvider)(nil)

type jwtClaims struct {
	SessionID string `json:"sid"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func NewAccessTokenProvider(issuer, secret string, ttl time.Duration) (AccessTokenProvider, error) {
	if issuer == "" {
		return nil, errors.New("JWT issuer is empty")
	}
	if len([]byte(secret)) < 32 {
		return nil, errors.New("JWT secret must contain at least 32 bytes")
	}
	if ttl <= 0 {
		return nil, errors.New("access token TTL must be positive")
	}

	return &jwtTokenProvider{
		issuer: issuer,
		secret: []byte(secret),
		ttl:    ttl,
	}, nil
}

func (p *jwtTokenProvider) Issue(claim AccessClaims) (*AccessToken, error) {
	if claim.UserID == 0 {
		return nil, ErrInvalidAccessToken
	}
	if _, err := uuid.Parse(claim.SessionID); err != nil {
		return nil, ErrInvalidAccessToken
	}

	now := time.Now().UTC()
	rawToken, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwtClaims{
			SessionID: claim.SessionID,
			TokenType: "access",
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    p.issuer,
				Subject:   strconv.FormatUint(uint64(claim.UserID), 10),
				IssuedAt:  jwt.NewNumericDate(now),
				NotBefore: jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(p.ttl)),
			},
		}).SignedString(p.secret)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	return &AccessToken{
		Value:     rawToken,
		ExpiresIn: int64(p.ttl / time.Second),
	}, nil
}

func (p *jwtTokenProvider) Verify(rawToken string) (*AccessClaims, error) {
	if rawToken == "" {
		return nil, ErrInvalidAccessToken
	}

	claim := &jwtClaims{}
	parsedToken, err := jwt.ParseWithClaims(
		rawToken,
		claim,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidAccessToken
			}
			return p.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(p.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5*time.Second),
	)

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidAccessToken, err)
	}

	if !parsedToken.Valid || claim.TokenType != "access" {
		return nil, ErrInvalidAccessToken
	}

	userID, err := strconv.ParseUint(claim.Subject, 10, strconv.IntSize)
	if err != nil || userID == 0 {
		return nil, ErrInvalidAccessToken
	}
	if _, err := uuid.Parse(claim.SessionID); err != nil {
		return nil, ErrInvalidAccessToken
	}

	return &AccessClaims{UserID: uint(userID), SessionID: claim.SessionID}, nil
}

type RefreshTokenIdentity struct {
	UserID    uint
	SessionID string
}

type RefreshTokenProvider interface {
	Generate(userID uint, sessionID string) (*RefreshToken, error)
	ParseIdentity(rawToken string) (*RefreshTokenIdentity, error)
	Match(rawToken string, expectedHash string) bool
}

type opaqueRefreshTokenProvider struct{}

var _ RefreshTokenProvider = (*opaqueRefreshTokenProvider)(nil)

func NewRefreshTokenProvider() RefreshTokenProvider {
	return &opaqueRefreshTokenProvider{}
}

func (p *opaqueRefreshTokenProvider) Generate(userID uint, sessionID string) (*RefreshToken, error) {
	if userID == 0 {
		return nil, ErrInvalidRefreshToken
	}
	if _, err := uuid.Parse(sessionID); err != nil {
		return nil, ErrInvalidRefreshToken
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	rawToken := strings.Join([]string{strconv.FormatUint(uint64(userID), 10), sessionID, base64.RawURLEncoding.EncodeToString(secret)}, ".")
	return &RefreshToken{
		Value:     rawToken,
		Hash:      hashRefreshToken(rawToken),
		UserID:    userID,
		SessionID: sessionID,
	}, nil
}

func (p *opaqueRefreshTokenProvider) ParseIdentity(rawToken string) (*RefreshTokenIdentity, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidRefreshToken
	}

	parsedUserID, err := strconv.ParseUint(parts[0], 10, strconv.IntSize)
	if err != nil || parsedUserID == 0 || parts[0] != strconv.FormatUint(parsedUserID, 10) {
		return nil, ErrInvalidRefreshToken
	}

	sessionID := parts[1]
	if _, err := uuid.Parse(sessionID); err != nil {
		return nil, ErrInvalidRefreshToken
	}

	secret, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(secret) != 32 {
		return nil, ErrInvalidRefreshToken
	}

	return &RefreshTokenIdentity{
		UserID:    uint(parsedUserID),
		SessionID: sessionID,
	}, nil
}

func (p *opaqueRefreshTokenProvider) Match(rawToken string, expectedHash string) bool {
	expected, err := hex.DecodeString(expectedHash)
	if err != nil || len(expected) != sha256.Size {
		return false
	}

	actual := sha256.Sum256([]byte(rawToken))

	return subtle.ConstantTimeCompare(
		actual[:],
		expected,
	) == 1
}

func hashRefreshToken(rawToken string) string {
	digest := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(digest[:])
}
