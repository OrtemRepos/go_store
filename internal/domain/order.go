package domain

import (
	"time"
	"github.com/OrtemRepos/go_store/internal/common/luhn"
)

type orderStatus string

const (
	REGISTERED orderStatus = "REGISTERED"
	PROCESSING orderStatus = "PROCESSING"
	INVALID    orderStatus = "INVALID"
	PROCESSED  orderStatus = "PROCESSED"
)

type Order struct {
	ID        uint         `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID    uint         `gorm:"not null;index" json:"-"`
	Number    string       `gorm:"uniqueIndex;not null" json:"number"`
	Accural   *int         `json:"accural,omitempty"`
	Completed bool         `gorm:"default:FALSE" json:"-"`
	Status    orderStatus  `json:"status"`
	CreatedAt time.Time    `gorm:"autoCreateTime" json:"created_at" time_format:"rfc3339"`
}

func NewOrder(number string, userID uint) (*Order, error) {
	order := Order{Number: number, UserID: userID, Status: REGISTERED}
	if !luhn.CheckValidNumber(number) {
		return nil, ErrInvalidOrderNubmer
	}
	return &order, nil
}