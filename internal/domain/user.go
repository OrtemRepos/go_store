package domain

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"index;unique" json:"email"`
	Password  string    `json:"password"`
	Orders    []*Order  `gorm:"foreignKey:UserID" json:"orders"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at,omitempty"`
}

func NewUser(email, passwordPlain string) (*User, error) {
	password, err := bcrypt.GenerateFromPassword([]byte(passwordPlain), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return &User{
		Email:    email,
		Password: string(password),
	}, nil
}

func (u *User) ValidatePassword(passwordPlain string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(passwordPlain))
	return err == nil
}

func (u *User) AddOrder(numberOrder string) (*Order, error) {
	for _, order := range u.Orders {
		if order.Number == numberOrder {
			return order, ErrOrderAlreadyExistsForUser
		}
	}
	order, err := NewOrder(numberOrder, u.ID)
	if err != nil {
		return nil, ErrInvalidOrderNubmer
	}
	u.Orders = append(u.Orders, order)
	return order, nil
}
