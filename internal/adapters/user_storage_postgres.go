package adapters

import (
	"errors"

	"github.com/OrtemRepos/go_store/internal/domain"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type UserStorageImpl struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewUserStorage(dsn string, logger *zap.Logger) (*UserStorageImpl, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto migrate the schema
	err = db.AutoMigrate(&domain.User{})
	if err != nil {
		return nil, err
	}

	return &UserStorageImpl{db: db, logger: logger}, nil
}

func (s *UserStorageImpl) GetByID(id int) (*domain.User, error) {
	var user domain.User
	result := s.db.First(&user, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.Join(domain.ErrUserNotExist, result.Error)
	} else if result.Error != nil {
		s.logger.Error("failed to get user by ID", zap.Int("id", id), zap.Error(result.Error))
		return nil, result.Error
	}
	return &user, nil
}

func (s *UserStorageImpl) GetByEmail(email string) (*domain.User, error) {
	var user domain.User
	result := s.db.Where("email = ?", email).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.Join(domain.ErrUserNotExist, result.Error)
	} else if result.Error != nil {
		s.logger.Error("failed to get user by ID", zap.String("email", email), zap.Error(result.Error))
		return nil, result.Error
	}
	return &user, nil
}

func (s *UserStorageImpl) Save(user *domain.User) error {
	result := s.db.Save(user)
	if result.Error != nil {
		s.logger.Error("failed to save user", zap.Error(result.Error))
		return result.Error
	}
	s.logger.Info("user saved successfully", zap.Uint("id", user.ID))
	return nil
}
