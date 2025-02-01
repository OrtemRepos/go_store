package domain

import (
	"strconv"
	"time"
)

type orderStatus int

const (
	NEW orderStatus = iota
	PROCESSING
	INVALID
	PROCESSED
)

type Order struct {
	ID        uint `gorm:"primaryKey;autoIncrement"`
	UserID    uint `gorm:"not null;index"`
	Number    string `gorm:"uniqueIndex;not null"`
	Status    orderStatus `json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at" time_format:"rfc3339"`
}

func (o *Order) CheckValidID() bool {
	sum := 0
	parity := len(o.Number) % 2
	for pos, char := range o.Number {
		digit, err := strconv.Atoi(string(char))
		if err != nil {
			return false
		}
		if pos % 2 == parity {
			digit *= 2
			if digit > 9 {
				digit = digit - 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}

func NewOrder(number string, userID uint) (*Order, error) {
	order := Order{Number: number, UserID: userID, Status: NEW}
	if !order.CheckValidID() {
		return nil, ErrInvalidOrderNubmer
	}
	return &order, nil
}