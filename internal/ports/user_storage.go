package ports

import "github.com/OrtemRepos/go_store/internal/domain"

type UserStorage interface {
	GetByID(id uint) (*domain.User, error)
	GetByEmail(email string) (*domain.User, error)
	Save(user *domain.User) error
}
