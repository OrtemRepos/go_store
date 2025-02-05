package adapters

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/OrtemRepos/go_store/configs"
	"github.com/OrtemRepos/go_store/internal/auth"
	"github.com/OrtemRepos/go_store/internal/domain"
	"github.com/OrtemRepos/go_store/internal/ports"
	"github.com/OrtemRepos/go_store/internal/service/order-service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RestAPI struct {
	logger       *zap.Logger
	jwt          ports.JWT
	userStorage  ports.UserStorage
	cfg          *configs.Config
	orderService *orderservice.OrderService
	*gin.Engine
}

func NewRestAPI(
	cfg *configs.Config,
	logger *zap.Logger,
	jwt ports.JWT,
	userStorage ports.UserStorage,
	enginge *gin.Engine,
	orderService *orderservice.OrderService,
) *RestAPI {
	return &RestAPI{
		logger:      logger,
		jwt:         jwt,
		userStorage: userStorage,
		cfg:         cfg,
		Engine:      enginge,
		orderService: orderService,
	}
}

func (r *RestAPI) Serve() {
	r.NoRoute(r.noPage)
	r.POST("/api/auth", r.authUser)
	r.POST("/api/register", r.registerUser)
	protectedRouter := r.Group("/api", auth.AuthMiddleware(r.jwt, r.logger))
	protectedRouter.POST("/user/orders", r.addOrder)
	protectedRouter.GET("/user/orders", r.getOrders)
	protectedRouter.GET("/user/balance", r.getBalance)
	protectedRouter.POST("/user/withdraw", r.newOrderWithdrawn)
	protectedRouter.GET("/user/withdraw", r.getWithdraws)

	r.orderService.Start(context.Background())

	err := r.Run("localhost:8080")
	if err != nil {
		r.logger.Error("error when starting the gin server", zap.Error(err))
	}
}

func (r *RestAPI) authUser(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	if email == "" || password == "" {
		c.AbortWithStatusJSON(
			http.StatusBadRequest,
			gin.H{
				"error": "empty password or email",
			},
		)
		return
	}
	user, err := r.userStorage.GetByEmail(email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatusJSON(
			http.StatusNotFound,
			gin.H{
				"error": fmt.Sprintf("user with email=%s does not exist", email),
			},
		)
		return
	} else if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if !user.ValidatePassword(password) {
		c.AbortWithStatusJSON(
			http.StatusUnauthorized,
			gin.H{"error": "wrong email or password"},
		)
		return
	}
	token, err := r.jwt.BuildJWTString(user.ID)
	if err != nil {
		r.logger.Error("error when creating a jwt-token", zap.Error(err))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.SetCookie("authGoOrder", token, r.cfg.Auth.TokenExp, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"UserID": user.ID, "msg": "successful authorization"})
}

func (r *RestAPI) registerUser(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	if email == "" || password == "" {
		c.AbortWithStatusJSON(
			http.StatusBadRequest,
			gin.H{
				"error": "empty password or email",
			},
		)
		return
	}
	user, err := domain.NewUser(email, password)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	err = r.userStorage.Save(user)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"UserID": user.ID, "msg": "registered user"})
}

func (r *RestAPI) addOrder(c *gin.Context) {
	userID := c.GetUint("UserID")
	number, ok := c.GetPostForm("number")
	if !ok {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	user, err := r.userStorage.GetByID(userID)
	if err != nil {
		r.logger.Error("can't get a user from the database", zap.Error(err))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	order, err := user.AddOrder(number)
	if errors.Is(err, domain.ErrInvalidOrderNubmer) {
		c.AbortWithStatus(http.StatusUnprocessableEntity)
		return
	} else if errors.Is(err, domain.ErrOrderAlreadyExistsForUser) {
		if !order.Completed {
			r.orderService.AsyncProcessOrder(c.Request.Context(), *order)
		}
		c.AbortWithStatus(http.StatusOK)
		return
	}
	err = r.userStorage.Save(user)
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		c.AbortWithStatus(http.StatusConflict)
		return
	} else if err != nil {
		r.logger.Error("error when saving to the database", zap.Error(err))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := r.orderService.AsyncProcessOrder(c.Request.Context(), *order); err != nil {
		c.AbortWithError(http.StatusTooManyRequests, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"User": user, "addedOrder": order})
}

func (r *RestAPI) getOrders(c *gin.Context) {
	userID := c.GetUint("UserID")
	user, err := r.userStorage.GetByID(userID)
	if err != nil {
		r.logger.Error(
			"error when retrieving a user from the database by id",
			zap.Uint("id", userID),
			zap.Error(err),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if len(user.Orders) == 0 {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": user.Orders})
}

func (r *RestAPI) getBalance(c *gin.Context) {
	userID := c.GetUint("UserID")
	balance, withdrawn, err := r.userStorage.UserBalance(userID)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"current": balance, "withdrawn": withdrawn})
}

func (r *RestAPI) newOrderWithdrawn(c *gin.Context) {
	userID := c.GetUint("UserID")
	sumString := c.PostForm("sum")
	number := c.PostForm("order")
	if number == "" {
		r.logger.Debug("empty number")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	sum, err := strconv.Atoi(sumString)
	if err != nil {
		r.logger.Debug("sum parsing error", zap.String("sum", sumString), zap.Error(err))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if sum <= 0 {
		r.logger.Debug("amount less than zero", zap.Int("sum", sum))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	user, err := r.userStorage.GetByID(userID)
	if err != nil {
		r.logger.Error(
			"error when retrieving a user from the database by id",
			zap.Uint("id", userID),
			zap.Error(err),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	
	withdrawn, err := user.AddWithdrawn(number, sum)
	if errors.Is(err, domain.ErrNotEnoughPoints) {
		c.AbortWithStatus(http.StatusPaymentRequired)
		return
	} else if errors.Is(err, domain.ErrOrderAlreadyExistsForUser) {
		c.AbortWithStatus(http.StatusAlreadyReported)
		return
	} else if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	err = r.userStorage.Save(user)
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		c.AbortWithStatus(http.StatusConflict)
		return
	} else if err != nil {
		r.logger.Warn("error when updating user data", zap.Error(err))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, withdrawn)
}

func (r *RestAPI) getWithdraws(c *gin.Context) {
	userID := c.GetUint("UserID")
	user, err := r.userStorage.GetByID(userID)
	if err != nil {
		r.logger.Error(
			"error when retrieving a user from the database by id",
			zap.Uint("id", userID),
			zap.Error(err),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if len(user.Withdraws) == 0 {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, user.Withdraws)
}

func (r *RestAPI) noPage(c *gin.Context) {
	c.String(http.StatusNotFound, "404 page not found")
}
