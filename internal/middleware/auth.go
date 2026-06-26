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
	ContextUserIDKey                = "authUserID"
	ContextUserRolesKey             = "authUserRoles"
	RuntimeConsoleWebSocketProtocol = "paap-runtime-console"
	EmbeddedProxyAuthCookieName     = "paap_proxy_token"
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

func UserIDFromRequest(r *http.Request) (uint, error) {
	if r == nil {
		return 0, errors.New("missing request")
	}
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		return UserIDFromAuthorization(r.Header.Get("Authorization"))
	}
	token := tokenFromWebSocketSubprotocols(r.Header.Get("Sec-WebSocket-Protocol"))
	if token != "" {
		return VerifyToken(token)
	}
	token = tokenFromEmbeddedProxyQuery(r)
	if token != "" {
		return VerifyToken(token)
	}
	token = tokenFromEmbeddedProxyCookie(r)
	if token != "" {
		return VerifyToken(token)
	}
	return 0, errors.New("missing token")
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
		userID, err := UserIDFromRequest(c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing token"})
			return
		}

		var user model.User
		if err := database.DB.First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		roles, err := model.UserRoleValues(database.DB, user.ID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "load user roles failed"})
			return
		}

		c.Set(ContextUserIDKey, user.ID)
		c.Set(ContextUserRolesKey, roles)
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

func tokenFromWebSocketSubprotocols(header string) string {
	parts := strings.Split(header, ",")
	for idx := 0; idx < len(parts)-1; idx++ {
		if strings.TrimSpace(parts[idx]) == RuntimeConsoleWebSocketProtocol {
			return strings.TrimSpace(parts[idx+1])
		}
	}
	return ""
}

func tokenFromEmbeddedProxyQuery(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	if !isEmbeddedProxyPath(r.URL.Path) {
		return ""
	}
	return strings.TrimSpace(r.URL.Query().Get("paap_token"))
}

func tokenFromEmbeddedProxyCookie(r *http.Request) string {
	if r == nil || r.URL == nil || !isEmbeddedProxyPath(r.URL.Path) {
		return ""
	}
	cookie, err := r.Cookie(EmbeddedProxyAuthCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func isEmbeddedProxyPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/environments/") && strings.Contains(path, "/proxy/")
}
