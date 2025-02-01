package auth

import (
	"go.uber.org/zap"

	"github.com/OrtemRepos/go_store/internal/ports"
)

func CheckToken(tokenString string, providerJWT ports.JWT, logger *zap.Logger) (*ports.Claims, error) {
	claims, err := providerJWT.GetClaims(tokenString)
	if err != nil {
		logger.Error("failed to validate token", zap.Error(err), zap.String("token", tokenString))
		return nil, err
	}
	logger.Info("user authorized successfully", zap.Any("claims", claims))
	return claims, nil
}
