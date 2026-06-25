package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"paap/config"
	"paap/internal/database"
	"paap/internal/middleware"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const keycloakStateCookie = "paap_keycloak_state"

type keycloakUserInfo map[string]interface{}

func KeycloakLogin(c *gin.Context) {
	cfg := config.Load()
	oauthConfig, ok := keycloakOAuthConfig(c, cfg)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "keycloak login is not configured"})
		return
	}
	state, err := randomHex(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     keycloakStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, oauthConfig.AuthCodeURL(state))
}

func KeycloakCallback(c *gin.Context) {
	cfg := config.Load()
	oauthConfig, ok := keycloakOAuthConfig(c, cfg)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "keycloak login is not configured"})
		return
	}
	stateCookie, err := c.Request.Cookie(keycloakStateCookie)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != c.Query("state") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid keycloak state"})
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: keycloakStateCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})

	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing keycloak code"})
		return
	}
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "keycloak token exchange failed"})
		return
	}
	info, err := fetchKeycloakUserInfo(cfg.KeycloakIssuerURL, token.AccessToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	roles := mergeKeycloakRoles(
		rolesFromKeycloakUserInfo(info, cfg.KeycloakClientID),
		rolesFromKeycloakAccessToken(token.AccessToken, cfg.KeycloakClientID),
	)
	user, roles, err := upsertKeycloakUser(info, roles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	paapToken, err := middleware.GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	redirectTo := "/login?token=" + url.QueryEscape(paapToken)
	if len(roles) > 0 {
		redirectTo += "&roles=" + url.QueryEscape(strings.Join(roles, ","))
	}
	c.Redirect(http.StatusFound, redirectTo)
}

func keycloakOAuthConfig(c *gin.Context, cfg *config.Config) (oauth2.Config, bool) {
	issuer := strings.TrimRight(strings.TrimSpace(cfg.KeycloakIssuerURL), "/")
	clientID := strings.TrimSpace(cfg.KeycloakClientID)
	if issuer == "" || clientID == "" {
		return oauth2.Config{}, false
	}
	return oauth2.Config{
		ClientID:     clientID,
		ClientSecret: cfg.KeycloakClientSecret,
		RedirectURL:  keycloakRedirectURL(c, cfg),
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  issuer + "/protocol/openid-connect/auth",
			TokenURL: issuer + "/protocol/openid-connect/token",
		},
	}, true
}

func keycloakRedirectURL(c *gin.Context, cfg *config.Config) string {
	if strings.TrimSpace(cfg.KeycloakRedirectURL) != "" {
		return strings.TrimSpace(cfg.KeycloakRedirectURL)
	}
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}
	return scheme + "://" + c.Request.Host + "/api/v1/auth/keycloak/callback"
}

func fetchKeycloakUserInfo(issuer, accessToken string) (keycloakUserInfo, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, strings.TrimRight(issuer, "/")+"/protocol/openid-connect/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("keycloak userinfo failed: %s", resp.Status)
	}
	var info keycloakUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return info, nil
}

func upsertKeycloakUser(info keycloakUserInfo, roles []string) (model.User, []string, error) {
	username := firstUserInfoString(info, "preferred_username", "email", "sub")
	if username == "" {
		return model.User{}, nil, fmt.Errorf("keycloak userinfo is missing username")
	}
	email := firstUserInfoString(info, "email")
	if email == "" {
		email = username + "@keycloak.local"
	}

	var user model.User
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("username = ?", username).First(&user).Error
		if err == gorm.ErrRecordNotFound {
			passwordHash, err := randomInitialUserPasswordHash()
			if err != nil {
				return err
			}
			user = model.User{Username: username, Email: email, Password: passwordHash}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if email != "" && user.Email != email {
			if err := tx.Model(&user).Update("email", email).Error; err != nil {
				return err
			}
			user.Email = email
		}
		_, err = model.ReplaceUserRoles(tx, user.ID, roles)
		return err
	})
	if err != nil {
		return model.User{}, nil, err
	}
	return user, roles, nil
}

func rolesFromKeycloakUserInfo(info keycloakUserInfo, clientID string) []string {
	roles := rolesFromKeycloakClaims(map[string]interface{}(info), clientID)
	if len(roles) == 0 {
		return []string{model.RoleUser}
	}
	return roles
}

func rolesFromKeycloakAccessToken(accessToken, clientID string) []string {
	parts := strings.Split(accessToken, ".")
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return rolesFromKeycloakClaims(claims, clientID)
}

func rolesFromKeycloakClaims(claims map[string]interface{}, clientID string) []string {
	roleSet := map[string]struct{}{}
	addRole := func(role string) {
		role = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(role)), "/")
		if model.IsValidUserRole(role) {
			roleSet[role] = struct{}{}
		}
	}
	if realmAccess, ok := claims["realm_access"].(map[string]interface{}); ok {
		addRolesFromInterface(realmAccess["roles"], addRole)
	}
	if resourceAccess, ok := claims["resource_access"].(map[string]interface{}); ok {
		if clientAccess, ok := resourceAccess[clientID].(map[string]interface{}); ok {
			addRolesFromInterface(clientAccess["roles"], addRole)
		}
	}
	addRolesFromInterface(claims["groups"], addRole)
	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	return roles
}

func mergeKeycloakRoles(roleLists ...[]string) []string {
	roleSet := map[string]struct{}{}
	for _, roles := range roleLists {
		for _, role := range roles {
			if model.IsValidUserRole(role) {
				roleSet[role] = struct{}{}
			}
		}
	}
	if len(roleSet) == 0 {
		return []string{model.RoleUser}
	}
	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	return roles
}

func addRolesFromInterface(value interface{}, add func(string)) {
	switch roles := value.(type) {
	case []interface{}:
		for _, role := range roles {
			if text, ok := role.(string); ok {
				add(text)
			}
		}
	case []string:
		for _, role := range roles {
			add(role)
		}
	}
}

func firstUserInfoString(info keycloakUserInfo, keys ...string) string {
	for _, key := range keys {
		if value, ok := info[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func randomInitialUserPasswordHash() (string, error) {
	value, err := randomHex(32)
	if err != nil {
		return "", err
	}
	return hashPassword(value)
}

func randomHex(size int) (string, error) {
	seed := make([]byte, size)
	if _, err := rand.Read(seed); err != nil {
		return "", err
	}
	return hex.EncodeToString(seed), nil
}
