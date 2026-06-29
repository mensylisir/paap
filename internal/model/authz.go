package model

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	ScopeSystem = "system"
	ScopeApp    = "app"
	ScopeEnv    = "env"

	RolePlatformAdmin = "platform_admin"
	RoleAppAdmin      = "app_admin"
	RoleUser          = "user"

	AppRoleAdmin  = "admin"
	AppRoleMember = "member"
	AppRoleViewer = "viewer"
)

type Permission struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	Code        string         `gorm:"uniqueIndex;size:100;not null" json:"code"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	Description string         `gorm:"size:500" json:"description"`
	ScopeType   string         `gorm:"size:20;not null;index" json:"scopeType"`
	Resource    string         `gorm:"size:60;not null" json:"resource"`
	Action      string         `gorm:"size:60;not null" json:"action"`
	GroupName   string         `gorm:"size:80" json:"groupName"`
	RiskLevel   string         `gorm:"size:20;default:normal" json:"riskLevel"`
	Builtin     bool           `gorm:"default:false;index" json:"builtin"`
	Enabled     bool           `gorm:"default:true;index" json:"enabled"`
}

type Role struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	Code        string         `gorm:"size:80;not null;uniqueIndex:idx_roles_code_scope" json:"code"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	Description string         `gorm:"size:500" json:"description"`
	ScopeType   string         `gorm:"size:20;not null;uniqueIndex:idx_roles_code_scope;index" json:"scopeType"`
	Builtin     bool           `gorm:"default:false;index" json:"builtin"`
	Editable    bool           `gorm:"default:true" json:"editable"`
	Enabled     bool           `gorm:"default:true;index" json:"enabled"`
	Permissions []Permission   `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

type RolePermission struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	RoleID       uint           `gorm:"not null;uniqueIndex:idx_role_permissions_role_permission" json:"roleId"`
	PermissionID uint           `gorm:"not null;uniqueIndex:idx_role_permissions_role_permission" json:"permissionId"`
}

type RoleBinding struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	UserID    uint           `gorm:"not null;index;uniqueIndex:idx_role_bindings_user_role_scope" json:"userId"`
	RoleID    uint           `gorm:"not null;index;uniqueIndex:idx_role_bindings_user_role_scope" json:"roleId"`
	Role      Role           `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	ScopeType string         `gorm:"size:20;not null;index;uniqueIndex:idx_role_bindings_user_role_scope" json:"scopeType"`
	ScopeID   uint           `gorm:"not null;default:0;index;uniqueIndex:idx_role_bindings_user_role_scope" json:"scopeId"`
	CreatedBy uint           `gorm:"default:0" json:"createdBy"`
}

func NormalizeScopeType(scopeType string) (string, error) {
	scopeType = strings.ToLower(strings.TrimSpace(scopeType))
	switch scopeType {
	case ScopeSystem, ScopeApp, ScopeEnv:
		return scopeType, nil
	default:
		return "", fmt.Errorf("invalid scope type")
	}
}
