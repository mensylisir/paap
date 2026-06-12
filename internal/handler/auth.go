package handler

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"paap/config"
	"paap/internal/database"
	"paap/internal/model"
)

// In-memory token storage for the local development server.
var tokenStore = make(map[string]uint)
var tokenMu sync.RWMutex

// For production JWT, use signed tokens; the local server uses opaque tokens.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateToken(userID uint) string {
	// Simple token: timestamp + userID
	return strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.Itoa(int(userID))
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

	token := generateToken(user.ID)
	tokenStore[token] = user.ID

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

	userID, ok := tokenStore[token]
	if !ok {
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
