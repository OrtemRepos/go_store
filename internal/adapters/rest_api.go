package adapters

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/OrtemRepos/go_store/configs"
	"github.com/OrtemRepos/go_store/internal/domain"
	"github.com/OrtemRepos/go_store/internal/ports"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
		logger: logger,
		jwt: jwt,
		userStorage: userStorage,
		cfg: cfg,
		Engine: enginge,
	}
}

func (r *RestAPI) Serve() {
	r.POST("/api/auth", r.authUser)
	r.POST("/api/register", r.registerUser)
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
	token, err := r.jwt.BuildJWTString(int(user.ID))
	if err != nil {
		r.logger.Error("error when creating a jwt-token", zap.Error(err))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.SetCookie("auth", token, r.cfg.Auth.TokenExp, "/auth", "safety", true, true)
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
	c.JSON(http.StatusCreated, gin.H{"UserID": user.ID, "msg": "registered registered user"})
}