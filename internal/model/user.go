package model

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	RolePlatformAdmin = "platform_admin"
	RoleAppAdmin      = "app_admin"
	RoleUser          = "user"
)

var validUserRoles = map[string]struct{}{
	RolePlatformAdmin: {},
	RoleAppAdmin:      {},
	RoleUser:          {},
}

type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Email     string         `gorm:"uniqueIndex;size:100" json:"email"`
	Password  string         `gorm:"size:255;not null" json:"-"`
	Roles     []UserRole     `gorm:"foreignKey:UserID" json:"roles,omitempty"`
}

type UserRole struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	UserID    uint           `gorm:"not null;uniqueIndex:idx_user_roles_user_role" json:"userId"`
	Role      string         `gorm:"size:30;not null;uniqueIndex:idx_user_roles_user_role" json:"role"`
}

func IsValidUserRole(role string) bool {
	_, ok := validUserRoles[role]
	return ok
}

func NormalizeUserRoles(roles []string) ([]string, error) {
	seen := make(map[string]struct{}, len(roles))
	normalized := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "" {
			continue
		}
		if !IsValidUserRole(role) {
			return nil, fmt.Errorf("invalid role: must be platform_admin, app_admin, or user")
		}
		if _, exists := seen[role]; exists {
			continue
		}
		seen[role] = struct{}{}
		normalized = append(normalized, role)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("at least one role is required")
	}
	return normalized, nil
}

func HasUserRole(roles []string, role string) bool {
	for _, candidate := range roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func UserRoleValues(db *gorm.DB, userID uint) ([]string, error) {
	var rows []UserRole
	if err := db.Where("user_id = ?", userID).Order("role").Find(&rows).Error; err != nil {
		return nil, err
	}
	roles := make([]string, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, row.Role)
	}
	return roles, nil
}

func ReplaceUserRoles(db *gorm.DB, userID uint, roles []string) ([]string, error) {
	normalized, err := NormalizeUserRoles(roles)
	if err != nil {
		return nil, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&UserRole{}).Error; err != nil {
			return err
		}
		rows := make([]UserRole, 0, len(normalized))
		for _, role := range normalized {
			rows = append(rows, UserRole{UserID: userID, Role: role})
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	return normalized, nil
}
