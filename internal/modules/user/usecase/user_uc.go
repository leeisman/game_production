package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/frankieli/game_product/internal/modules/user/domain"
	"github.com/frankieli/game_product/internal/modules/user/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// UserUseCase handles user-related business logic (registration, authentication, sessions)
type UserUseCase struct {
	userRepo      *repository.UserRepository
	sessionRepo   *repository.SessionRepository
	jwtSecret     []byte
	tokenDuration time.Duration
}

// NewUserUseCase creates a new user use case
func NewUserUseCase(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	jwtSecret string,
	tokenDuration time.Duration,
) *UserUseCase {
	return &UserUseCase{
		userRepo:      userRepo,
		sessionRepo:   sessionRepo,
		jwtSecret:     []byte(jwtSecret),
		tokenDuration: tokenDuration,
	}
}

// Register registers a new user
func (uc *UserUseCase) Register(ctx context.Context, username, password, email string) (int64, error) {
	// Validate input
	if username == "" || password == "" || email == "" {
		return 0, fmt.Errorf("username, password, and email are required")
	}

	if len(password) < 6 {
		return 0, fmt.Errorf("password must be at least 6 characters")
	}

	// Check if username exists
	exists, err := uc.userRepo.UsernameExists(ctx, username)
	if err != nil {
		return 0, fmt.Errorf("failed to check username: %w", err)
	}
	if exists {
		return 0, fmt.Errorf("username already exists")
	}

	// Check if email exists
	exists, err = uc.userRepo.EmailExists(ctx, email)
	if err != nil {
		return 0, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return 0, fmt.Errorf("email already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &domain.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Email:        email,
		Status:       domain.UserStatusActive,
	}

	err = uc.userRepo.Create(ctx, user)
	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	return user.UserID, nil
}

// Login authenticates a user and returns a JWT token
func (uc *UserUseCase) Login(ctx context.Context, username, password string) (int64, string, string, time.Time, error) {
	// Get user by username
	user, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return 0, "", "", time.Time{}, fmt.Errorf("invalid username or password")
	}

	// Check if user is active
	if !user.IsActive() {
		return 0, "", "", time.Time{}, fmt.Errorf("user account is not active")
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return 0, "", "", time.Time{}, fmt.Errorf("invalid username or password")
	}

	// Generate JWT token
	token, expiresAt, err := uc.generateToken(user.UserID, user.Username)
	if err != nil {
		return 0, "", "", time.Time{}, fmt.Errorf("failed to generate token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := uc.generateRefreshToken()
	if err != nil {
		return 0, "", "", time.Time{}, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session
	session := &domain.Session{
		SessionID: refreshToken,
		UserID:    user.UserID,
		Token:     token,
		ExpiresAt: expiresAt.Add(24 * time.Hour * 7), // Refresh token valid for 7 days
	}

	err = uc.sessionRepo.Create(ctx, session)
	if err != nil {
		return 0, "", "", time.Time{}, fmt.Errorf("failed to create session: %w", err)
	}

	// Update last login
	_ = uc.userRepo.UpdateLastLogin(ctx, user.UserID)

	return user.UserID, token, refreshToken, expiresAt, nil
}

// ValidateToken validates a JWT token
func (uc *UserUseCase) ValidateToken(ctx context.Context, tokenString string) (int64, string, time.Time, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return uc.jwtSecret, nil
	})
	if err != nil {
		return 0, "", time.Time{}, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return 0, "", time.Time{}, fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", time.Time{}, fmt.Errorf("invalid token claims")
	}

	userID := int64(claims["user_id"].(float64))
	username := claims["username"].(string)
	exp := int64(claims["exp"].(float64))
	expiresAt := time.Unix(exp, 0)

	return userID, username, expiresAt, nil
}

// Logout logs out a user by invalidating their session
func (uc *UserUseCase) Logout(ctx context.Context, token string) error {
	// In a real implementation, you might want to blacklist the token
	// For now, we'll just delete sessions associated with this token
	return uc.sessionRepo.DeleteByToken(ctx, token)
}

// RefreshToken generates a new access token using a refresh token
func (uc *UserUseCase) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	// Get session by refresh token (session_id)
	session, err := uc.sessionRepo.GetBySessionID(ctx, refreshToken)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("invalid refresh token")
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		_ = uc.sessionRepo.Delete(ctx, session.SessionID)
		return "", "", time.Time{}, fmt.Errorf("refresh token expired")
	}

	// Get user
	user, err := uc.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("user not found")
	}

	if !user.IsActive() {
		return "", "", time.Time{}, fmt.Errorf("user account is not active")
	}

	// Generate new access token
	newToken, expiresAt, err := uc.generateToken(user.UserID, user.Username)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to generate token: %w", err)
	}

	// Update session with new token
	session.Token = newToken
	err = uc.sessionRepo.Update(ctx, session)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to update session: %w", err)
	}

	return newToken, refreshToken, expiresAt, nil
}

// generateToken generates a JWT token
func (uc *UserUseCase) generateToken(userID int64, username string) (string, time.Time, error) {
	expiresAt := time.Now().Add(uc.tokenDuration)

	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      expiresAt.Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(uc.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// generateRefreshToken generates a random refresh token
func (uc *UserUseCase) generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
