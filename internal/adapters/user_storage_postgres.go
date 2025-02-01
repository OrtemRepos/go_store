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
	db, err := gorm.Open(
		postgres.Open(dsn),
		&gorm.Config{
			PrepareStmt: true,
			TranslateError: true,
		},
	)
	if err != nil {
		return nil, err
	}

	// Auto migrate the schema
	err = db.AutoMigrate(&domain.User{}, &domain.Order{})
	if err != nil {
		return nil, err
	}

	return &UserStorageImpl{db: db, logger: logger}, nil
}

func (s *UserStorageImpl) GetByID(id uint) (*domain.User, error) {
	var user domain.User
	result := s.db.Model(&domain.User{}).
		Preload("Orders", func(db *gorm.DB) *gorm.DB { return db.Order("orders.created_at ASC") }).
		Where("id = ?", id).
		First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.Join(domain.ErrUserNotExist, result.Error)
	} else if result.Error != nil {
		s.logger.Error("failed to get user by ID", zap.Uint("id", id), zap.Error(result.Error))
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
