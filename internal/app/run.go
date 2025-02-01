package app

import (
	"fmt"
	"os"

	"github.com/OrtemRepos/go_store/configs"
	"github.com/OrtemRepos/go_store/internal/adapters"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
	userStorage, err := adapters.NewUserStorage(dsn, logger)
	if err != nil {
		logger.Fatal("can't create userStorage", zap.String("dsn", dsn), zap.Error(err))
		return err
	}
	jwt := adapters.NewProviderJWT(cfg, logger)

	router := gin.Default()

	restAPI := adapters.NewRestAPI(
		cfg, logger, jwt, userStorage, router,
	)

	restAPI.Serve()
	return nil
}