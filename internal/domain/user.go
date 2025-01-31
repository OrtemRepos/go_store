package domain

import (
	"golang.org/x/crypto/bcrypt"
	"time"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"index;unique" json:"email"`
	Password  string    `json:"password"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"create_at"`
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
