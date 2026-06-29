package model

import "testing"

func TestHasUserRole(t *testing.T) {
	roles := []string{RoleUser, RoleAppAdmin}

	if !HasUserRole(roles, RoleAppAdmin) {
		t.Fatalf("expected role %q to be present", RoleAppAdmin)
	}
	if HasUserRole(roles, RolePlatformAdmin) {
		t.Fatalf("did not expect role %q to be present", RolePlatformAdmin)
	}
}
