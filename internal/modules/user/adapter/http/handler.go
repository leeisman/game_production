package http

import (
	"net/http"
	"time"

	"github.com/frankieli/game_product/internal/modules/user/usecase"

	"github.com/frankieli/game_product/pkg/logger"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// Handler handles HTTP requests for user module
type Handler struct {
	userUC          *usecase.UserUseCase
	loginLimiter    *rate.Limiter
	registerLimiter *rate.Limiter
}

// NewHandler creates a new HTTP handler
func NewHandler(userUC *usecase.UserUseCase) *Handler {
	return &Handler{
		userUC: userUC,
		// Login: 100 RPS, Burst 50
		loginLimiter: rate.NewLimiter(rate.Limit(100), 50),
		// Register: 50 RPS, Burst 20 (Register is heavier and less frequent)
		registerLimiter: rate.NewLimiter(rate.Limit(50), 20),
	}
}

// RegisterRoutes registers all user routes to the given router group
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/register", h.Register)
	router.POST("/login", h.Login)
	router.POST("/logout", h.Logout)
	router.GET("/health", h.Health)
}

// Server represents the User module HTTP server
type Server struct {
	handler *Handler
	engine  *gin.Engine
	port    string
}

// NewServer creates a new independent HTTP server for User module
func NewServer(handler *Handler, port string) *Server {
	r := gin.Default()

	// Register routes under /api/users by default for standalone mode
	// Or we can make the prefix configurable if needed
	api := r.Group("/api")
	users := api.Group("/users")
	handler.RegisterRoutes(users)

	return &Server{
		handler: handler,
		engine:  r,
		port:    port,
	}
}

// Run starts the HTTP server
func (s *Server) Run() error {
	return s.engine.Run(":" + s.port)
}

// DTOs
type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"required,email"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type registerResponse struct {
	UserID  int64 `json:"user_id"`
	Success bool  `json:"success"`
}

type loginResponse struct {
	UserID       int64  `json:"user_id"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
}

// Register handles user registration
func (h *Handler) Register(c *gin.Context) {
	// Rate Limiting Check
	if !h.registerLimiter.Allow() {
		logger.Warn(c.Request.Context()).Msg("Register: rate limit exceeded")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many registration requests, please try again later"})
		return
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn(c.Request.Context()).Err(err).Msg("Register: invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := h.userUC.Register(c.Request.Context(), req.Username, req.Password, req.Email)
	if err != nil {
		logger.Error(c.Request.Context()).Err(err).Str("username", req.Username).Msg("Register: failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Info(c.Request.Context()).Int64("user_id", userID).Str("username", req.Username).Msg("Register: success")

	c.JSON(http.StatusOK, registerResponse{
		UserID:  userID,
		Success: true,
	})
}

// Login handles user login
func (h *Handler) Login(c *gin.Context) {
	// Rate Limiting Check
	if !h.loginLimiter.Allow() {
		logger.Warn(c.Request.Context()).Msg("Login: rate limit exceeded")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login requests, please try again later"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn(c.Request.Context()).Err(err).Msg("Login: invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, token, refreshToken, expiresAt, err := h.userUC.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		logger.Error(c.Request.Context()).Err(err).Str("username", req.Username).Msg("Login: failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	logger.Info(c.Request.Context()).Int64("user_id", userID).Str("username", req.Username).Msg("Login: success")

	c.JSON(http.StatusOK, loginResponse{
		UserID:       userID,
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt.Format(time.RFC3339),
	})
}

// Logout handles user logout
func (h *Handler) Logout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		logger.Warn(c.Request.Context()).Msg("Logout: missing token")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
		return
	}

	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	err := h.userUC.Logout(c.Request.Context(), token)
	if err != nil {
		logger.Error(c.Request.Context()).Err(err).Msg("Logout: failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Info(c.Request.Context()).Msg("Logout: success")

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Health handles health checks
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "user-service"})
}
