package ports

import (
	"github.com/golang-jwt/jwt/v5"
)

type JWT interface {
	BuildJWTString(id uint) (string, error)
	GetClaims(tokenString string) (*Claims, error)
}
type Claims struct {
	jwt.RegisteredClaims
	UserID uint
}
