package service

import (
	"errors"
	"fmt"
	"strings"

	"paap/internal/authz"
	"paap/internal/model"

	"gorm.io/gorm"
)

var (
	ErrRoleNotFound     = errors.New("role not found")
	ErrRoleNotEditable  = errors.New("built-in roles cannot be modified")
	ErrRoleNotDeletable = errors.New("built-in roles cannot be deleted")
	ErrRoleAssigned     = errors.New("role is assigned to users")
)

type RoleItem struct {
	ID              uint     `json:"id"`
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	ScopeType       string   `json:"scopeType"`
	Builtin         bool     `json:"builtin"`
	Editable        bool     `json:"editable"`
	Enabled         bool     `json:"enabled"`
	PermissionIDs   []uint   `json:"permissionIds,omitempty"`
	PermissionCodes []string `json:"permissionCodes,omitempty"`
}

type SaveRoleInput struct {
	Code          string
	Name          string
	Description   string
	ScopeType     string
	Enabled       *bool
	PermissionIDs []uint
}

func ListAssignableRoles(db *gorm.DB, scopeType string) ([]RoleItem, error) {
	query := db.Model(&model.Role{}).Where("enabled = ?", true)
	if strings.TrimSpace(scopeType) != "" {
		normalized, err := model.NormalizeScopeType(scopeType)
		if err != nil {
			return nil, ValidationError{Message: err.Error()}
		}
		query = query.Where("scope_type = ?", normalized)
	}

	var roles []model.Role
	if err := query.Order("scope_type, builtin DESC, code").Find(&roles).Error; err != nil {
		return nil, err
	}
	items := make([]RoleItem, 0, len(roles))
	for _, role := range roles {
		items = append(items, roleToItem(role, false))
	}
	return items, nil
}

func ListRoles(db *gorm.DB, scopeType string) ([]RoleItem, error) {
	query := db.Preload("Permissions").Order("scope_type, builtin DESC, code")
	if strings.TrimSpace(scopeType) != "" {
		normalized, err := model.NormalizeScopeType(scopeType)
		if err != nil {
			return nil, ValidationError{Message: err.Error()}
		}
		query = query.Where("scope_type = ?", normalized)
	}

	var roles []model.Role
	if err := query.Find(&roles).Error; err != nil {
		return nil, err
	}
	items := make([]RoleItem, 0, len(roles))
	for _, role := range roles {
		items = append(items, roleToItem(role, true))
	}
	return items, nil
}

func CreateRole(db *gorm.DB, input SaveRoleInput) (RoleItem, error) {
	scopeType, err := model.NormalizeScopeType(input.ScopeType)
	if err != nil {
		return RoleItem{}, ValidationError{Message: err.Error()}
	}
	code, err := normalizeRoleCode(input.Code)
	if err != nil {
		return RoleItem{}, ValidationError{Message: err.Error()}
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	role := model.Role{
		Code:        code,
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		ScopeType:   scopeType,
		Builtin:     false,
		Editable:    true,
		Enabled:     enabled,
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replaceRolePermissions(tx, role, input.PermissionIDs)
	}); err != nil {
		return RoleItem{}, err
	}
	if err := db.Preload("Permissions").First(&role, role.ID).Error; err != nil {
		return RoleItem{}, err
	}
	return roleToItem(role, true), nil
}

func UpdateRole(db *gorm.DB, roleID uint, input SaveRoleInput) (RoleItem, error) {
	var role model.Role
	if err := db.First(&role, roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RoleItem{}, ErrRoleNotFound
		}
		return RoleItem{}, err
	}
	if role.Builtin || !role.Editable {
		return RoleItem{}, ErrRoleNotEditable
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"name":        strings.TrimSpace(input.Name),
			"description": strings.TrimSpace(input.Description),
		}
		if input.Enabled != nil {
			updates["enabled"] = *input.Enabled
		}
		if err := tx.Model(&role).Updates(updates).Error; err != nil {
			return err
		}
		role.Name = updates["name"].(string)
		role.Description = updates["description"].(string)
		if input.Enabled != nil {
			role.Enabled = *input.Enabled
		}
		return replaceRolePermissions(tx, role, input.PermissionIDs)
	}); err != nil {
		return RoleItem{}, err
	}
	if err := authz.InvalidateRoleBindings(db, role.ID); err != nil {
		return RoleItem{}, err
	}
	if err := db.Preload("Permissions").First(&role, role.ID).Error; err != nil {
		return RoleItem{}, err
	}
	return roleToItem(role, true), nil
}

func DeleteRole(db *gorm.DB, roleID uint) error {
	var role model.Role
	if err := db.First(&role, roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return err
	}
	if role.Builtin || !role.Editable {
		return ErrRoleNotDeletable
	}
	var bindingCount int64
	if err := db.Model(&model.RoleBinding{}).Where("role_id = ?", role.ID).Count(&bindingCount).Error; err != nil {
		return err
	}
	if bindingCount > 0 {
		return ErrRoleAssigned
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("role_id = ?", role.ID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		return tx.Delete(&role).Error
	})
}

func replaceRolePermissions(tx *gorm.DB, role model.Role, permissionIDs []uint) error {
	normalized := normalizeRolePermissionIDs(permissionIDs)
	if len(normalized) == 0 {
		return ValidationError{Message: "at least one permission is required"}
	}
	var permissions []model.Permission
	if err := tx.Where("id IN ? AND enabled = ?", normalized, true).Find(&permissions).Error; err != nil {
		return err
	}
	if len(permissions) != len(normalized) {
		return ValidationError{Message: "invalid permission id"}
	}
	for _, permission := range permissions {
		if !roleCanUsePermission(role.ScopeType, permission.ScopeType) {
			return ValidationError{Message: fmt.Sprintf("permission %s cannot be assigned to %s role", permission.Code, role.ScopeType)}
		}
	}
	if err := tx.Unscoped().Where("role_id = ?", role.ID).Delete(&model.RolePermission{}).Error; err != nil {
		return err
	}
	rows := make([]model.RolePermission, 0, len(normalized))
	for _, permissionID := range normalized {
		rows = append(rows, model.RolePermission{RoleID: role.ID, PermissionID: permissionID})
	}
	return tx.Create(&rows).Error
}

func normalizeRolePermissionIDs(permissionIDs []uint) []uint {
	seen := make(map[uint]struct{}, len(permissionIDs))
	normalized := make([]uint, 0, len(permissionIDs))
	for _, id := range permissionIDs {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

func roleCanUsePermission(roleScopeType, permissionScopeType string) bool {
	switch roleScopeType {
	case model.ScopeSystem:
		return permissionScopeType == model.ScopeSystem
	case model.ScopeApp:
		return permissionScopeType == model.ScopeApp || permissionScopeType == model.ScopeEnv
	case model.ScopeEnv:
		return permissionScopeType == model.ScopeEnv
	default:
		return false
	}
}

func roleToItem(role model.Role, includePermissions bool) RoleItem {
	item := RoleItem{
		ID:          role.ID,
		Code:        role.Code,
		Name:        role.Name,
		Description: role.Description,
		ScopeType:   role.ScopeType,
		Builtin:     role.Builtin,
		Editable:    role.Editable,
		Enabled:     role.Enabled,
	}
	if includePermissions {
		item.PermissionIDs = make([]uint, 0, len(role.Permissions))
		item.PermissionCodes = make([]string, 0, len(role.Permissions))
		for _, permission := range role.Permissions {
			item.PermissionIDs = append(item.PermissionIDs, permission.ID)
			item.PermissionCodes = append(item.PermissionCodes, permission.Code)
		}
	}
	return item
}

func normalizeRoleCode(code string) (string, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return "", fmt.Errorf("role code is required")
	}
	if len(code) > 80 {
		return "", fmt.Errorf("role code is too long")
	}
	for _, ch := range code {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.' {
			continue
		}
		return "", fmt.Errorf("role code may only contain lowercase letters, numbers, dot, dash, and underscore")
	}
	return code, nil
}
