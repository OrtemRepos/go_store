package adapters

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/OrtemRepos/go_store/configs"
	"github.com/OrtemRepos/go_store/internal/auth"
	"github.com/OrtemRepos/go_store/internal/domain"
	"github.com/OrtemRepos/go_store/internal/ports"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RestAPI struct {
	logger      *zap.Logger
	jwt         ports.JWT
	userStorage ports.UserStorage
	cfg         *configs.Config
	*gin.Engine
}

func NewRestAPI(
	cfg *configs.Config,
	logger *zap.Logger,
	jwt ports.JWT,
	userStorage ports.UserStorage,
	enginge *gin.Engine,
) *RestAPI {
	return &RestAPI{
		logger:      logger,
		jwt:         jwt,
		userStorage: userStorage,
		cfg:         cfg,
		Engine:      enginge,
	}
}

func (r *RestAPI) Serve() {
	r.NoRoute(r.noPage)
	r.POST("/api/auth", r.authUser)
	r.POST("/api/register", r.registerUser)
	protectedRouter := r.Group("/api", auth.AuthMiddleware(r.jwt, r.logger))
	protectedRouter.POST("/user/orders", r.addOrder)
	protectedRouter.GET("/user/orders", r.getOrders)
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
	if errors.Is(err, domain.ErrUserNotExist) {
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
	}
	c.JSON(http.StatusOK, gin.H{"orders": user.Orders})
}

func (r *RestAPI) noPage(c *gin.Context) {
	c.String(http.StatusNotFound, "404 page not found")
}
