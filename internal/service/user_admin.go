package service

import (
	"errors"

	"paap/internal/authz"
	"paap/internal/model"

	"gorm.io/gorm"
)

func ListPlatformUsers(db *gorm.DB) ([]AuthUser, error) {
	var users []model.User
	if err := db.Order("id asc").Find(&users).Error; err != nil {
		return nil, err
	}
	items := make([]AuthUser, 0, len(users))
	for _, user := range users {
		item, err := authUserFromModel(db, user)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func UpdatePlatformUserRoles(db *gorm.DB, userID uint, roleCodes []string, createdBy uint) (AuthUser, error) {
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AuthUser{}, ErrUserNotFound
		}
		return AuthUser{}, err
	}
	roles, err := authz.ReplaceSystemRoleBindings(db, userID, roleCodes, createdBy)
	if err != nil {
		return AuthUser{}, err
	}
	return AuthUser{ID: user.ID, Username: user.Username, Email: user.Email, Roles: roles}, nil
}
