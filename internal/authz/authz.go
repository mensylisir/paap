package authz

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"paap/internal/model"

	"gorm.io/gorm"
)

type Scope struct {
	Type string
	ID   uint
}

const permissionCacheTTL = 5 * time.Minute

type permissionCacheEntry struct {
	userID    uint
	expiresAt time.Time
	codeSet   map[string]struct{}
}

var permissionCache = struct {
	sync.RWMutex
	items map[string]permissionCacheEntry
}{
	items: map[string]permissionCacheEntry{},
}

func SystemScope() Scope {
	return Scope{Type: model.ScopeSystem}
}

func AppScope(appID uint) Scope {
	return Scope{Type: model.ScopeApp, ID: appID}
}

func EnvScope(envID uint) Scope {
	return Scope{Type: model.ScopeEnv, ID: envID}
}

func Can(db *gorm.DB, userID uint, scope Scope, permissionCode string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database is nil")
	}
	if userID == 0 {
		return false, nil
	}
	if permissionCode == "" {
		return false, nil
	}
	codeSet, err := PermissionCodeSet(db, userID, scope)
	if err != nil {
		return false, err
	}
	_, ok := codeSet[permissionCode]
	return ok, nil
}

func BindRole(db *gorm.DB, userID uint, roleCode string, scope Scope, createdBy uint) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	var role model.Role
	if err := db.Where("code = ? AND scope_type = ? AND enabled = ?", roleCode, scope.Type, true).First(&role).Error; err != nil {
		return err
	}
	binding := model.RoleBinding{
		UserID:    userID,
		RoleID:    role.ID,
		ScopeType: scope.Type,
		ScopeID:   scope.ID,
		CreatedBy: createdBy,
	}
	if err := db.Where("user_id = ? AND role_id = ? AND scope_type = ? AND scope_id = ?", userID, role.ID, scope.Type, scope.ID).
		FirstOrCreate(&binding).Error; err != nil {
		return err
	}
	InvalidateUser(userID)
	return nil
}

func ReplaceSystemRoleBindings(db *gorm.DB, userID uint, roleCodes []string, createdBy uint) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	normalized, err := normalizeEnabledRoleCodes(db, roleCodes, model.ScopeSystem)
	if err != nil {
		return nil, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Where("user_id = ? AND scope_type = ? AND scope_id = ?", userID, model.ScopeSystem, 0).
			Delete(&model.RoleBinding{}).Error; err != nil {
			return err
		}
		for _, code := range normalized {
			var role model.Role
			if err := tx.Where("code = ? AND scope_type = ? AND enabled = ?", code, model.ScopeSystem, true).First(&role).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.RoleBinding{
				UserID:    userID,
				RoleID:    role.ID,
				ScopeType: model.ScopeSystem,
				ScopeID:   0,
				CreatedBy: createdBy,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	InvalidateUser(userID)
	return normalized, nil
}

func ReplaceRolePermissions(db *gorm.DB, roleID uint, permissionIDs []uint) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if roleID == 0 {
		return fmt.Errorf("role id is required")
	}

	var role model.Role
	if err := db.First(&role, roleID).Error; err != nil {
		return err
	}
	if role.Builtin || !role.Editable {
		return fmt.Errorf("built-in roles cannot be modified")
	}

	normalized := normalizePermissionIDs(permissionIDs)
	if len(normalized) == 0 {
		return fmt.Errorf("at least one permission is required")
	}
	var permissions []model.Permission
	if err := db.Where("id IN ? AND enabled = ?", normalized, true).Find(&permissions).Error; err != nil {
		return err
	}
	if len(permissions) != len(normalized) {
		return fmt.Errorf("invalid permission id")
	}
	for _, perm := range permissions {
		if !roleCanUsePermission(role.ScopeType, perm.ScopeType) {
			return fmt.Errorf("permission %s cannot be assigned to %s role", perm.Code, role.ScopeType)
		}
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		rows := make([]model.RolePermission, 0, len(normalized))
		for _, permissionID := range normalized {
			rows = append(rows, model.RolePermission{RoleID: roleID, PermissionID: permissionID})
		}
		return tx.Create(&rows).Error
	}); err != nil {
		return err
	}
	return InvalidateRoleBindings(db, roleID)
}

func ReplaceAppRoleBinding(db *gorm.DB, userID uint, appID uint, roleCode string, createdBy uint) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.Where("code = ? AND scope_type = ? AND enabled = ?", roleCode, model.ScopeApp, true).First(&role).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().
			Where("user_id = ? AND scope_type = ? AND scope_id = ?", userID, model.ScopeApp, appID).
			Delete(&model.RoleBinding{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.RoleBinding{
			UserID:    userID,
			RoleID:    role.ID,
			ScopeType: model.ScopeApp,
			ScopeID:   appID,
			CreatedBy: createdBy,
		}).Error
	}); err != nil {
		return err
	}
	InvalidateUser(userID)
	return nil
}

func RemoveAppRoleBindings(db *gorm.DB, userID uint, appID uint) error {
	if err := db.Unscoped().
		Where("user_id = ? AND scope_type = ? AND scope_id = ?", userID, model.ScopeApp, appID).
		Delete(&model.RoleBinding{}).Error; err != nil {
		return err
	}
	InvalidateUser(userID)
	return nil
}

func normalizeEnabledRoleCodes(db *gorm.DB, roleCodes []string, scopeType string) ([]string, error) {
	seen := make(map[string]struct{}, len(roleCodes))
	normalized := make([]string, 0, len(roleCodes))
	for _, code := range roleCodes {
		code = strings.ToLower(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("at least one role is required")
	}
	var count int64
	if err := db.Model(&model.Role{}).
		Where("code IN ? AND scope_type = ? AND enabled = ?", normalized, scopeType, true).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if int(count) != len(normalized) {
		return nil, fmt.Errorf("invalid role")
	}
	sort.Strings(normalized)
	return normalized, nil
}

func normalizePermissionIDs(permissionIDs []uint) []uint {
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
	sort.Slice(normalized, func(i, j int) bool { return normalized[i] < normalized[j] })
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

func SystemRoleCodes(db *gorm.DB, userID uint) ([]string, error) {
	var roles []model.Role
	if err := db.
		Joins("JOIN role_bindings ON role_bindings.role_id = roles.id AND role_bindings.deleted_at IS NULL").
		Where("role_bindings.user_id = ? AND role_bindings.scope_type = ? AND role_bindings.scope_id = ? AND roles.enabled = ? AND roles.deleted_at IS NULL", userID, model.ScopeSystem, 0, true).
		Order("roles.code").
		Find(&roles).Error; err != nil {
		return nil, err
	}
	result := make([]string, 0, len(roles))
	for _, role := range roles {
		result = append(result, role.Code)
	}
	return result, nil
}

func PermissionCodes(db *gorm.DB, userID uint, scope Scope) ([]string, error) {
	codeSet, err := PermissionCodeSet(db, userID, scope)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(codeSet))
	for code := range codeSet {
		result = append(result, code)
	}
	sort.Strings(result)
	return result, nil
}

func PermissionCodeSet(db *gorm.DB, userID uint, scope Scope) (map[string]struct{}, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if userID == 0 {
		return map[string]struct{}{}, nil
	}
	cacheKey := permissionCacheKey(db, userID, scope)
	now := time.Now()
	permissionCache.RLock()
	cached, ok := permissionCache.items[cacheKey]
	if ok && cached.expiresAt.After(now) {
		result := copyCodeSet(cached.codeSet)
		permissionCache.RUnlock()
		return result, nil
	}
	permissionCache.RUnlock()

	result, err := permissionCodesFromDB(db, userID, scope)
	if err != nil {
		return nil, err
	}
	permissionCache.Lock()
	permissionCache.items[cacheKey] = permissionCacheEntry{
		userID:    userID,
		expiresAt: now.Add(permissionCacheTTL),
		codeSet:   copyCodeSet(result),
	}
	permissionCache.Unlock()
	return result, nil
}

func InvalidateUser(userID uint) {
	if userID == 0 {
		return
	}
	permissionCache.Lock()
	for key, entry := range permissionCache.items {
		if entry.userID == userID {
			delete(permissionCache.items, key)
		}
	}
	permissionCache.Unlock()
}

func InvalidateUsers(userIDs []uint) {
	seen := map[uint]struct{}{}
	for _, userID := range userIDs {
		if userID > 0 {
			seen[userID] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return
	}
	permissionCache.Lock()
	for key, entry := range permissionCache.items {
		if _, ok := seen[entry.userID]; ok {
			delete(permissionCache.items, key)
		}
	}
	permissionCache.Unlock()
}

func InvalidateRoleBindings(db *gorm.DB, roleID uint) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if roleID == 0 {
		return nil
	}
	var userIDs []uint
	if err := db.Model(&model.RoleBinding{}).
		Where("role_id = ?", roleID).
		Distinct().
		Pluck("user_id", &userIDs).Error; err != nil {
		return err
	}
	InvalidateUsers(userIDs)
	return nil
}

func InvalidateAll() {
	permissionCache.Lock()
	permissionCache.items = map[string]permissionCacheEntry{}
	permissionCache.Unlock()
}

func permissionCodesFromDB(db *gorm.DB, userID uint, scope Scope) (map[string]struct{}, error) {
	codes := map[string]struct{}{}
	add := func(items []string) {
		for _, item := range items {
			codes[item] = struct{}{}
		}
	}

	platformAdmin, err := hasRoleBinding(db, userID, model.RolePlatformAdmin, model.ScopeSystem, 0)
	if err != nil {
		return nil, err
	}
	if platformAdmin {
		allCodes, err := allEnabledPermissionCodes(db)
		if err != nil {
			return nil, err
		}
		add(allCodes)
		return codes, nil
	}

	systemCodes, err := permissionCodesInScope(db, userID, model.ScopeSystem, 0)
	if err != nil {
		return nil, err
	}
	add(systemCodes)

	switch scope.Type {
	case model.ScopeSystem:
	case model.ScopeApp:
		appCodes, err := permissionCodesInScope(db, userID, model.ScopeApp, scope.ID)
		if err != nil {
			return nil, err
		}
		add(appCodes)
	case model.ScopeEnv:
		envCodes, err := permissionCodesInScope(db, userID, model.ScopeEnv, scope.ID)
		if err != nil {
			return nil, err
		}
		add(envCodes)
		var env model.Environment
		if err := db.Select("id", "application_id").First(&env, scope.ID).Error; err != nil {
			return nil, err
		}
		appCodes, err := permissionCodesInScope(db, userID, model.ScopeApp, env.ApplicationID)
		if err != nil {
			return nil, err
		}
		add(appCodes)
	default:
		return nil, fmt.Errorf("invalid scope type %q", scope.Type)
	}
	return codes, nil
}

func permissionCodesInScope(db *gorm.DB, userID uint, scopeType string, scopeID uint) ([]string, error) {
	var codes []string
	err := db.Table("permissions").
		Select("DISTINCT permissions.code").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id AND role_permissions.deleted_at IS NULL").
		Joins("JOIN roles ON roles.id = role_permissions.role_id AND roles.deleted_at IS NULL AND roles.enabled = TRUE").
		Joins("JOIN role_bindings ON role_bindings.role_id = roles.id AND role_bindings.deleted_at IS NULL").
		Where("permissions.deleted_at IS NULL AND permissions.enabled = TRUE").
		Where("role_bindings.user_id = ? AND role_bindings.scope_type = ? AND role_bindings.scope_id = ?", userID, scopeType, scopeID).
		Order("permissions.code").
		Pluck("permissions.code", &codes).Error
	return codes, err
}

func allEnabledPermissionCodes(db *gorm.DB) ([]string, error) {
	var codes []string
	err := db.Model(&model.Permission{}).
		Where("enabled = ?", true).
		Order("code").
		Pluck("code", &codes).Error
	return codes, err
}

func hasRoleBinding(db *gorm.DB, userID uint, roleCode, scopeType string, scopeID uint) (bool, error) {
	var count int64
	err := db.Table("role_bindings").
		Joins("JOIN roles ON roles.id = role_bindings.role_id AND roles.deleted_at IS NULL AND roles.enabled = TRUE").
		Where("role_bindings.deleted_at IS NULL").
		Where("role_bindings.user_id = ? AND role_bindings.scope_type = ? AND role_bindings.scope_id = ?", userID, scopeType, scopeID).
		Where("roles.code = ?", roleCode).
		Count(&count).Error
	return count > 0, err
}

func permissionCacheKey(db *gorm.DB, userID uint, scope Scope) string {
	return fmt.Sprintf("%p:%d:%s:%d", db, userID, scope.Type, scope.ID)
}

func copyCodeSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for code := range source {
		result[code] = struct{}{}
	}
	return result
}
