package handler

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"paap/internal/model"
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
