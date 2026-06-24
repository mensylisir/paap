package middleware

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

	"paap/config"
	"paap/internal/database"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
)

const (
	ContextUserIDKey   = "authUserID"
	ContextUserRoleKey = "authUserRole"
)

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

func GenerateToken(userID uint) (string, error) {
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

func UserIDFromAuthorization(header string) (uint, error) {
	token := parseBearerToken(header)
	if token == "" {
		return 0, errors.New("missing token")
	}
	return VerifyToken(token)
}

func VerifyToken(token string) (uint, error) {
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

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := UserIDFromAuthorization(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing token"})
			return
		}

		var user model.User
		if err := database.DB.First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		c.Set(ContextUserIDKey, user.ID)
		c.Set(ContextUserRoleKey, user.Role)
		c.Next()
	}
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
