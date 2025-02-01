package auth

import (
	"net/http"

	"github.com/OrtemRepos/go_store/internal/ports"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func AuthMiddleware(providerJWT ports.JWT, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := c.GetStringMap("result")
		if result == nil {
			result = make(map[string]interface{})
		}
		tokenString, err := c.Cookie("authGoOrder")
		if err != nil || tokenString == "" {
			logger.Error("authorization failed: no auth cookie", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization failed: no auth cookie"})
			return
		}

		claims, err := CheckToken(tokenString, providerJWT, logger)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "BAD CREND"})
			return
		}
		if claims.UserID == 0 {
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{"error": "Empty UserID"},
			)
			return
		}
		c.Set("claims", claims)
		c.Set("UserID", claims.UserID)
		result["UserID"] = claims.UserID
		c.Set("result", result)
		c.Next()
	}
}
