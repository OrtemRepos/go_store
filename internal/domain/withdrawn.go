package domain

import (
	"time"

	"github.com/OrtemRepos/go_store/internal/common/luhn"
)

type Withdraw struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	Number    string    `gorm:"uniqueIndex;not null" json:"number"`
	UserID    string    `gorm:"not null;index" json:"-"`
	Sum       int       `json:"sum"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at" time_format:"rfc3339"`
}

func NewWithdraw(number string, sum int) (*Withdraw, error) {
	if !luhn.CheckValidNumber(number) {
		return nil, ErrInvalidOrderNubmer
	}
	return &Withdraw{
		Number: number,
		Sum: sum,
	}, nil
}