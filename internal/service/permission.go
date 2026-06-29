package service

import (
	"paap/internal/authz"
	"paap/internal/model"

	"gorm.io/gorm"
)

type CurrentPermissions struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

type PermissionTreeGroup struct {
	Group       string             `json:"group"`
	ScopeType   string             `json:"scopeType"`
	Permissions []model.Permission `json:"permissions"`
}

func CurrentPermissionsForUser(db *gorm.DB, userID uint, scope authz.Scope) (CurrentPermissions, error) {
	roles, err := authz.SystemRoleCodes(db, userID)
	if err != nil {
		return CurrentPermissions{}, err
	}
	permissions, err := authz.PermissionCodes(db, userID, scope)
	if err != nil {
		return CurrentPermissions{}, err
	}
	return CurrentPermissions{Roles: roles, Permissions: permissions}, nil
}

func ListPermissionTree(db *gorm.DB, scopeType string) ([]PermissionTreeGroup, error) {
	query := db.Where("enabled = ?", true).Order("scope_type, group_name, code")
	if scopeType != "" {
		normalized, err := model.NormalizeScopeType(scopeType)
		if err != nil {
			return nil, ValidationError{Message: err.Error()}
		}
		query = query.Where("scope_type = ?", normalized)
	}
	var rows []model.Permission
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	groups := make([]PermissionTreeGroup, 0)
	index := map[string]int{}
	for _, row := range rows {
		key := row.ScopeType + "/" + row.GroupName
		pos, ok := index[key]
		if !ok {
			pos = len(groups)
			index[key] = pos
			groups = append(groups, PermissionTreeGroup{Group: row.GroupName, ScopeType: row.ScopeType})
		}
		groups[pos].Permissions = append(groups[pos].Permissions, row)
	}
	return groups, nil
}
