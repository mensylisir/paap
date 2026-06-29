package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"paap/config"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
)

func TestRolesFromKeycloakUserInfoMapsRealmClientAndGroupRoles(t *testing.T) {
	roles := rolesFromKeycloakUserInfo(keycloakUserInfo{
		"realm_access": map[string]interface{}{
			"roles": []interface{}{"platform_admin", "offline_access"},
		},
		"resource_access": map[string]interface{}{
			"paap": map[string]interface{}{
				"roles": []interface{}{"app_admin"},
			},
		},
		"groups": []interface{}{"/user"},
	}, "paap")

	for _, want := range []string{model.RolePlatformAdmin, model.RoleAppAdmin, model.RoleUser} {
		if !model.HasUserRole(roles, want) {
			t.Fatalf("roles = %#v, missing %s", roles, want)
		}
	}
}

func TestRolesFromKeycloakUserInfoDefaultsToUser(t *testing.T) {
	roles := rolesFromKeycloakUserInfo(keycloakUserInfo{}, "paap")

	if len(roles) != 1 || roles[0] != model.RoleUser {
		t.Fatalf("roles = %#v, want user", roles)
	}
}

func TestRolesFromKeycloakAccessTokenMapsRealmAndClientRoles(t *testing.T) {
	token := unsignedJWT(t, map[string]interface{}{
		"realm_access": map[string]interface{}{
			"roles": []interface{}{"platform_admin", "offline_access"},
		},
		"resource_access": map[string]interface{}{
			"paap": map[string]interface{}{
				"roles": []interface{}{"app_admin"},
			},
		},
	})

	roles := rolesFromKeycloakAccessToken(token, "paap")

	if !model.HasUserRole(roles, model.RolePlatformAdmin) || !model.HasUserRole(roles, model.RoleAppAdmin) {
		t.Fatalf("roles = %#v, want platform_admin and app_admin", roles)
	}
}

func TestKeycloakIssuerURLResolvesHostnameTemplate(t *testing.T) {
	c := keycloakTestContext(t, "http://paap.local.test:30091/api/v1/auth/keycloak/login")
	cfg := &config.Config{
		KeycloakIssuerURL: "http://{hostname}:30080/realms/paap",
		KeycloakClientID:  "paap",
	}

	oauthConfig, ok := keycloakOAuthConfig(c, cfg)
	if !ok {
		t.Fatalf("keycloak config should be enabled")
	}

	if oauthConfig.Endpoint.AuthURL != "http://paap.local.test:30080/realms/paap/protocol/openid-connect/auth" {
		t.Fatalf("AuthURL = %q", oauthConfig.Endpoint.AuthURL)
	}
	if oauthConfig.RedirectURL != "http://paap.local.test:30091/api/v1/auth/keycloak/callback" {
		t.Fatalf("RedirectURL = %q", oauthConfig.RedirectURL)
	}
}

func TestKeycloakRedirectURLUsesForwardedHeadersAndTemplate(t *testing.T) {
	c := keycloakTestContext(t, "http://paap-server.paap-system/api/v1/auth/keycloak/login")
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Header.Set("X-Forwarded-Host", "paap.example.test")
	cfg := &config.Config{
		KeycloakIssuerURL:   "https://auth.example.test/realms/paap",
		KeycloakClientID:    "paap",
		KeycloakRedirectURL: "{scheme}://{host}/api/v1/auth/keycloak/callback",
	}

	oauthConfig, ok := keycloakOAuthConfig(c, cfg)
	if !ok {
		t.Fatalf("keycloak config should be enabled")
	}

	if oauthConfig.RedirectURL != "https://paap.example.test/api/v1/auth/keycloak/callback" {
		t.Fatalf("RedirectURL = %q", oauthConfig.RedirectURL)
	}
}

func TestKeycloakOAuthConfigUsesPublicAuthURLAndBackchannelTokenURL(t *testing.T) {
	c := keycloakTestContext(t, "http://paap.local.test:30091/api/v1/auth/keycloak/login")
	cfg := &config.Config{
		KeycloakIssuerURL:            "https://auth.example.test/realms/paap",
		KeycloakBackchannelIssuerURL: "http://paap-keycloak.paap-system.svc.cluster.local:8080/realms/paap",
		KeycloakClientID:             "paap",
	}

	oauthConfig, ok := keycloakOAuthConfig(c, cfg)
	if !ok {
		t.Fatalf("keycloak config should be enabled")
	}

	if oauthConfig.Endpoint.AuthURL != "https://auth.example.test/realms/paap/protocol/openid-connect/auth" {
		t.Fatalf("AuthURL = %q", oauthConfig.Endpoint.AuthURL)
	}
	if oauthConfig.Endpoint.TokenURL != "http://paap-keycloak.paap-system.svc.cluster.local:8080/realms/paap/protocol/openid-connect/token" {
		t.Fatalf("TokenURL = %q", oauthConfig.Endpoint.TokenURL)
	}
	issuer, ok := keycloakBackchannelIssuerURL(c, cfg)
	if !ok || issuer != "http://paap-keycloak.paap-system.svc.cluster.local:8080/realms/paap" {
		t.Fatalf("backchannel issuer = %q, ok=%v", issuer, ok)
	}
}

func unsignedJWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	header, err := json.Marshal(map[string]interface{}{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
}

func keycloakTestContext(t *testing.T, rawURL string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, rawURL, nil)
	c.Request = req
	return c
}
