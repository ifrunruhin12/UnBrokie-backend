package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifrunruhin12/money-manager/internal/handler"
)

func NewRouter(
	jwtSecret string,
	rateLimitRPM int,
	logger *slog.Logger,
	db *pgxpool.Pool,
	authH *handler.AuthHandler,
	accountH *handler.AccountHandler,
	transactionH *handler.TransactionHandler,
	categoryH *handler.CategoryHandler,
	bigBuyH *handler.BigBuyHandler,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestLogger(logger))

	r.GET("/health", func(c *gin.Context) {
		response := gin.H{
			"status":  "ok",
			"service": "money-manager",
		}

		if db != nil {
			if err := db.Ping(c.Request.Context()); err != nil {
				response["status"] = "degraded"
				response["database"] = "unreachable"
				c.JSON(503, response)
				return
			}
			response["database"] = "ok"
		}

		c.JSON(200, response)
	})

	r.GET("/swagger", handler.ServeSwaggerUI())
	r.StaticFile("/swagger.yaml", "./docs/swagger.yaml")

	v1 := r.Group("/api/v1")

	auth := v1.Group("/auth")
	{
		auth.POST("/register", authH.Register)
		auth.POST("/login", authH.Login)
	}

	// Protected routes — all require a valid JWT
	protected := v1.Group("")
	protected.Use(AuthMiddleware(jwtSecret))
	{
		protected.GET("/balance", accountH.GetBalance)
		protected.PATCH("/account/balance", accountH.UpdateStartingBalance)
		protected.PATCH("/account/timezone", accountH.UpdateTimezone)
		protected.POST("/account/reconcile", accountH.Reconcile)

		protected.POST("/transactions", RateLimiter(rateLimitRPM), transactionH.Create)
		protected.GET("/transactions", transactionH.List)
		protected.PATCH("/transactions/:id/override", transactionH.Override)
		protected.PATCH("/transactions/:id/skip", transactionH.Skip)
		protected.PATCH("/transactions/:id/restore", transactionH.Restore)
		protected.GET("/transactions/:id/history", transactionH.GetHistory)

		protected.POST("/categories", categoryH.Create)
		protected.GET("/categories", categoryH.List)
		protected.PATCH("/categories/:id", categoryH.Update)
		protected.DELETE("/categories/:id", categoryH.Delete)

		protected.POST("/big-buys", bigBuyH.Create)
		protected.GET("/big-buys", bigBuyH.List)
		protected.PATCH("/big-buys/:id", bigBuyH.Update)
		protected.DELETE("/big-buys/:id", bigBuyH.Delete)
	}

	return r
}
