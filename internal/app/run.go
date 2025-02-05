package app

import (
	"fmt"
	"os"

	"github.com/OrtemRepos/go_store/configs"
	"github.com/OrtemRepos/go_store/internal/adapters"
	"github.com/OrtemRepos/go_store/internal/service/order-service"
	"github.com/OrtemRepos/go_store/internal/worker-pool"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)



func Run() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Errorf("cant't create logger: %w", err))
	}
	defer func() { _ = logger.Sync() }()
	cfg, err := configs.GetConfig(os.Args[1:])
	if err != nil {
		logger.Fatal("can't read the config", zap.Error(err))
		return err
	}
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		cfg.Database.Host, cfg.Database.User, cfg.Database.Password,
		cfg.Database.Dbname, cfg.Database.Port,
	)
	db, err := gorm.Open(
		postgres.Open(dsn),
		&gorm.Config{
			PrepareStmt: true,
			TranslateError: true,
		},
	)
	if err != nil {
		logger.Error("error while opening the database", zap.Error(err))
	}
	userStorage := adapters.NewUserStorage(db, logger)
	if err != nil {
		logger.Fatal("can't create userStorage", zap.String("dsn", dsn), zap.Error(err))
		return err
	}
	jwt := adapters.NewProviderJWT(cfg, logger)

	router := gin.Default()

	const (
		workerCount = 5
		bufferSize = 100
		errMaximumAmount = 100
		maxRetries = 5
		retryDelay = 1000
	)
	poolMetrics := worker.NewPoolMetrics()

	wp := worker.NewWorkerPool(
		"OrderWP",
		workerCount, bufferSize, errMaximumAmount,
		poolMetrics, worker.NewWorkerMetrics,
		logger,
	)

	orderService, err := orderservice.NewOrderService(
		db, logger, wp, userStorage, cfg.Server.AccuralSystemAddress,
		maxRetries, retryDelay,
	)
	if err != nil {
		logger.Fatal("ошмбка при создании OrderService", zap.Error(err))
	}

	restAPI := adapters.NewRestAPI(
		cfg, logger, jwt, userStorage, router,
		orderService,
	)

	restAPI.Serve()
	return nil
}