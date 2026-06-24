package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"paap/config"
	"paap/internal/database"
	"paap/internal/model"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type jwtClaims struct {
	UserID    uint   `json:"uid"`
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateToken(userID uint) (string, error) {
	now := time.Now().Unix()
	header := jwtHeader{Algorithm: "HS256", Type: "JWT"}
	claims := jwtClaims{
		UserID:    userID,
		Subject:   strconv.Itoa(int(userID)),
		IssuedAt:  now,
		ExpiresAt: now + int64(24*time.Hour/time.Second),
	}

	headerSegment, err := encodeJWTPart(header)
	if err != nil {
		return "", err
	}
	claimsSegment, err := encodeJWTPart(claims)
	if err != nil {
		return "", err
	}
	signingInput := headerSegment + "." + claimsSegment
	return signingInput + "." + signJWT(signingInput), nil
}

func encodeJWTPart(value interface{}) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func signJWT(signingInput string) string {
	mac := hmac.New(sha256.New, []byte(config.Load().JWTSecret))
	mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func parseBearerToken(header string) string {
	value := strings.TrimSpace(header)
	if len(value) >= 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return value
}

func verifyToken(token string) (uint, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, errors.New("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSignature := signJWT(signingInput)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSignature)) {
		return 0, errors.New("invalid token signature")
	}

	headerData, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, fmt.Errorf("decode header: %w", err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerData, &header); err != nil {
		return 0, fmt.Errorf("decode header json: %w", err)
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return 0, errors.New("unsupported token header")
	}

	claimsData, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("decode claims: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsData, &claims); err != nil {
		return 0, fmt.Errorf("decode claims json: %w", err)
	}
	if claims.UserID == 0 {
		return 0, errors.New("missing user id")
	}
	if claims.ExpiresAt < time.Now().Unix() {
		return 0, errors.New("token expired")
	}
	return claims.UserID, nil
}

// Register creates a new user
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	user := model.User{
		Username: req.Username,
		Password: hash,
		Email:    req.Email,
		Role:     "user",
	}

	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// Login authenticates a user and returns a token
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if !checkPasswordHash(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token":    token,
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

// GetCurrentUser returns the current authenticated user
func GetCurrentUser(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	userID, err := verifyToken(parseBearerToken(token))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	var user model.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// SeedDefaultUsers creates local default users when the database is empty.
func SeedDefaultUsers() {
	var count int64
	database.DB.Model(&model.User{}).Count(&count)
	if count > 0 {
		return
	}

	hash, _ := hashPassword("admin123")
	admin := model.User{
		Username: "admin",
		Email:    "admin@paap.local",
		Password: hash,
		Role:     "admin",
	}
	database.DB.Create(&admin)

	hash, _ = hashPassword("user123")
	user := model.User{
		Username: "user",
		Email:    "user@paap.local",
		Password: hash,
		Role:     "user",
	}
	database.DB.Create(&user)
}

func init() {
	// Seed JWT secret check
	_ = config.Load().JWTSecret
}
