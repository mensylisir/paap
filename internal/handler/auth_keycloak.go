package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"paap/config"
	"paap/internal/database"
	"paap/internal/middleware"
	"paap/internal/model"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
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
	issuer, ok := keycloakBackchannelIssuerURL(c, cfg)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "keycloak login is not configured"})
		return
	}
	info, err := fetchKeycloakUserInfo(issuer, token.AccessToken)
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
	issuer, ok := keycloakIssuerURL(c, cfg)
	backchannelIssuer, backchannelOK := keycloakBackchannelIssuerURL(c, cfg)
	clientID := strings.TrimSpace(cfg.KeycloakClientID)
	if !ok || clientID == "" {
		return oauth2.Config{}, false
	}
	if !backchannelOK {
		backchannelIssuer = issuer
	}
	return oauth2.Config{
		ClientID:     clientID,
		ClientSecret: cfg.KeycloakClientSecret,
		RedirectURL:  keycloakRedirectURL(c, cfg),
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  issuer + "/protocol/openid-connect/auth",
			TokenURL: backchannelIssuer + "/protocol/openid-connect/token",
		},
	}, true
}

func keycloakIssuerURL(c *gin.Context, cfg *config.Config) (string, bool) {
	issuer := strings.TrimRight(resolveKeycloakURLTemplate(c, cfg.KeycloakIssuerURL), "/")
	return issuer, issuer != ""
}

func keycloakBackchannelIssuerURL(c *gin.Context, cfg *config.Config) (string, bool) {
	issuer := strings.TrimRight(resolveKeycloakURLTemplate(c, cfg.KeycloakBackchannelIssuerURL), "/")
	if issuer != "" {
		return issuer, true
	}
	return keycloakIssuerURL(c, cfg)
}

func keycloakRedirectURL(c *gin.Context, cfg *config.Config) string {
	if strings.TrimSpace(cfg.KeycloakRedirectURL) != "" {
		return resolveKeycloakURLTemplate(c, cfg.KeycloakRedirectURL)
	}
	return keycloakExternalScheme(c) + "://" + keycloakExternalHost(c) + "/api/v1/auth/keycloak/callback"
}

func resolveKeycloakURLTemplate(c *gin.Context, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || c == nil {
		return value
	}
	fullHost := keycloakExternalHost(c)
	replacer := strings.NewReplacer(
		"{scheme}", keycloakExternalScheme(c),
		"{host}", fullHost,
		"{hostname}", hostWithoutPort(fullHost),
	)
	return replacer.Replace(value)
}

func keycloakExternalScheme(c *gin.Context) string {
	if c == nil {
		return "http"
	}
	if scheme := firstHeaderValue(c.GetHeader("X-Forwarded-Proto")); scheme != "" {
		return scheme
	}
	if c.Request != nil && c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

func keycloakExternalHost(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if host := firstHeaderValue(c.GetHeader("X-Forwarded-Host")); host != "" {
		return host
	}
	if host := firstHeaderValue(c.GetHeader("X-Forwarded-Server")); host != "" {
		return host
	}
	return c.Request.Host
}

func firstHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[0])
}

func hostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(hostname, "[]")
	}
	if strings.Count(host, ":") == 1 {
		if idx := strings.LastIndex(host, ":"); idx > 0 {
			return host[:idx]
		}
	}
	return strings.Trim(host, "[]")
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
	return service.UpsertKeycloakUser(database.DB, map[string]interface{}(info), roles)
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
		if isBuiltInSystemRoleCode(role) {
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
			if isBuiltInSystemRoleCode(role) {
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

func isBuiltInSystemRoleCode(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case model.RolePlatformAdmin, model.RoleAppAdmin, model.RoleUser:
		return true
	default:
		return false
	}
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

func randomHex(size int) (string, error) {
	seed := make([]byte, size)
	if _, err := rand.Read(seed); err != nil {
		return "", err
	}
	return hex.EncodeToString(seed), nil
}
