package adapters

import (
	"errors"

	"github.com/OrtemRepos/go_store/internal/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserStorageImpl struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewUserStorage(db *gorm.DB, logger *zap.Logger) *UserStorageImpl {
	err := db.AutoMigrate(domain.User{}, domain.Order{}, domain.Withdraw{})
	if err != nil {
		;logger.Fatal("migration error", zap.Error(err))
	}
	return &UserStorageImpl{db: db, logger: logger}
}

func (s *UserStorageImpl) GetByID(id uint) (*domain.User, error) {
	var user domain.User
	result := s.db.Model(&domain.User{}).
		Preload("Orders", func(db *gorm.DB) *gorm.DB { return db.Order("orders.created_at ASC") }).
		Preload("Withdraws", func(db *gorm.DB) *gorm.DB { return db.Order("withdraws.created_at ASC")}).
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
	result := s.db.Model(&user).Where("email = ?", email).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.Join(domain.ErrUserNotExist, result.Error)
	} else if result.Error != nil {
		s.logger.Error("failed to get user by ID", zap.String("email", email), zap.Error(result.Error))
		return nil, result.Error
	}
	return &user, nil
}

func (s *UserStorageImpl) AddAccural(id uint, accural int) error {
	var err error
	user := domain.User{ID: id}
	stmt := s.db.Model(&user).Update("current_balance", gorm.Expr("current_balance + ?", accural))
	if accural < 0 {
		err = stmt.Update("withdrawn", gorm.Expr("withdrawn - ?", accural)).Error
	}
	if err != nil {
		s.logger.Warn("error when update accural", zap.Error(err))
	}
	return nil
}

func (s *UserStorageImpl) UserBalance(id uint) (int, int, error) {
	user := domain.User{ID: id}
	err := s.db.Model(&user).Select("current_balance", "withdrawn").First(&user).Error
	if err != nil {
		s.logger.Warn("balance error", zap.Error(err))
		return 0, 0, err
	}
	return user.CurrentBalance, user.Withdrawn, nil
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
