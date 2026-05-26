package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/ifrunruhin12/money-manager/internal/api"
	"github.com/ifrunruhin12/money-manager/internal/config"
	"github.com/ifrunruhin12/money-manager/internal/db"
	"github.com/ifrunruhin12/money-manager/internal/handler"
	"github.com/ifrunruhin12/money-manager/internal/repository"
	"github.com/ifrunruhin12/money-manager/internal/service"
	"github.com/ifrunruhin12/money-manager/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "err", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("configuration loaded", "port", cfg.Port, "log_level", cfg.LogLevel)

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		log.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	log.Info("database connection established")

	if err := db.RunMigrations(cfg.DatabaseURL, "migrations", log); err != nil {
		log.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(pool)
	accountRepo := repository.NewAccountRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	transactionRepo := repository.NewTransactionRepository(pool)
	bigBuyRepo := repository.NewBigBuyRepository(pool)

	authService := service.NewAuthService(pool, userRepo, accountRepo, categoryRepo, cfg)
	balanceService := service.NewBalanceService(accountRepo, transactionRepo, bigBuyRepo, cfg.BalanceStalenessThreshold, log)
	categoryService := service.NewCategoryService(categoryRepo, pool)
	transactionService := service.NewTransactionService(pool, transactionRepo, accountRepo, cfg.EnableEventLog, log)
	bigBuyService := service.NewBigBuyService(pool, bigBuyRepo, accountRepo, cfg.EnableEventLog, log)

	authHandler := handler.NewAuthHandler(authService)
	accountHandler := handler.NewAccountHandler(balanceService, accountRepo)
	transactionHandler := handler.NewTransactionHandler(transactionService)
	categoryHandler := handler.NewCategoryHandler(categoryService)
	bigBuyHandler := handler.NewBigBuyHandler(bigBuyService)

	router := api.NewRouter(
		cfg.JWTSecret,
		cfg.RateLimitRPM,
		log,
		pool, // Pass DB pool for health check
		authHandler,
		accountHandler,
		transactionHandler,
		categoryHandler,
		bigBuyHandler,
	)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Info("starting HTTP server", "addr", addr)
	if err := router.Run(addr); err != nil {
		log.Error("failed to start server", "err", err)
		os.Exit(1)
	}
}
