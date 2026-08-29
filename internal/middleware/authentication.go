package middleware

import (
	"fmt"
	"mmchat/internal/api"
	"mmchat/internal/service"
	"strings"

	"github.com/gin-gonic/gin"
)

const PrincipalKey = "auth_principal"

func Authentication(verifier service.AccessTokenVerifier, sessionService service.SessionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			_ = c.Error(api.ErrUnauthenticated)
			c.Abort()
			return
		}

		claims, err := verifier.Verify(parts[1])
		if err != nil {
			_ = c.Error(api.ErrUnauthenticated)
			c.Abort()
			return
		}

		session, err := sessionService.GetSession(c.Request.Context(), claims.SessionID)
		if err != nil {
			_ = c.Error(fmt.Errorf("get authentication session: %w", err))
			c.Abort()
			return
		}

		if session == nil {
			_ = c.Error(api.ErrUnauthenticated)
			c.Abort()
			return
		}

		c.Set(PrincipalKey, claims)
		c.Next()
	}
}

func Principal(c *gin.Context) (*service.AccessClaims, error) {
	value, exists := c.Get(PrincipalKey)
	if !exists {
		return nil, api.ErrUnauthenticated
	}
	principal := value.(*service.AccessClaims)
	return principal, nil
}
