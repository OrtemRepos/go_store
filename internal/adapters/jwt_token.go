package adapters

import (
	"errors"
	"fmt"
	"time"

	"github.com/OrtemRepos/go_store/configs"
	"github.com/OrtemRepos/go_store/internal/ports"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type ProviderJWT struct {
	tokenExp  time.Duration
	logger    *zap.Logger
	secretKey []byte
}

func NewProviderJWT(cfg *configs.Config, logger *zap.Logger) *ProviderJWT {
	return &ProviderJWT{
		tokenExp:  time.Duration(cfg.Auth.TokenExp) * time.Second,
		secretKey: []byte(cfg.Auth.SecretKey),
		logger:    logger,
	}
}

var (
	ErrNotValidToken = errors.New("not valid token")
)

func (pj *ProviderJWT) BuildJWTString(id int) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		ports.Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(pj.tokenExp)),
			},
			UserID: id,
		},
	)

	tokenString, err := token.SignedString(pj.secretKey)
	if err != nil {
		pj.logger.Error("Failed to sign token", zap.Error(err))
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (pj *ProviderJWT) GetClaims(tokenString string) (*ports.Claims, error) {
	claims := &ports.Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return pj.secretKey, nil
		},
	)
	if err != nil {
		pj.logger.Debug("Failed to parse token claims", zap.Error(err))
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	if !token.Valid {
		pj.logger.Warn("Invalid token received")
		return nil, ErrNotValidToken
	}

	return claims, nil
}