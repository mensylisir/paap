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
	ErrApplicationNotFound = errors.New("application not found")
	ErrAppMemberNotFound   = errors.New("application member not found")
	ErrAppMemberExists     = errors.New("user is already an application member")
	ErrInvalidAppRole      = errors.New("invalid member role")
	ErrLastAppAdmin        = errors.New("application must keep at least one admin member")
	ErrUserNotFound        = errors.New("user not found")
)

func ListApplicationMembers(db *gorm.DB, appID uint) ([]model.AppMember, error) {
	if _, err := findApplication(db, appID); err != nil {
		return nil, err
	}
	var members []model.AppMember
	if err := db.Where("application_id = ?", appID).
		Preload("User").
		Order("id").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func InviteApplicationMember(db *gorm.DB, appID uint, username string, role string, createdBy uint) (model.AppMember, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return model.AppMember{}, fmt.Errorf("username is required")
	}

	var created model.AppMember
	err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := findApplication(tx, appID); err != nil {
			return err
		}
		normalizedRole, err := normalizeAppRole(tx, role)
		if err != nil {
			return err
		}
		var user model.User
		if err := tx.Where("username = ?", username).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		var existing int64
		if err := tx.Model(&model.AppMember{}).
			Where("application_id = ? AND user_id = ?", appID, user.ID).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrAppMemberExists
		}
		created = model.AppMember{ApplicationID: appID, UserID: user.ID, Role: normalizedRole}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if err := authz.ReplaceAppRoleBinding(tx, user.ID, appID, normalizedRole, createdBy); err != nil {
			return err
		}
		return tx.Preload("User").First(&created, created.ID).Error
	})
	return created, err
}

func UpdateApplicationMemberRole(db *gorm.DB, appID uint, memberID uint, role string, createdBy uint) (model.AppMember, error) {
	var member model.AppMember
	err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := findApplication(tx, appID); err != nil {
			return err
		}
		normalizedRole, err := normalizeAppRole(tx, role)
		if err != nil {
			return err
		}
		if err := tx.Where("application_id = ? AND id = ?", appID, memberID).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAppMemberNotFound
			}
			return err
		}
		if member.Role == model.AppRoleAdmin && normalizedRole != model.AppRoleAdmin {
			count, err := applicationAdminCount(tx, appID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return ErrLastAppAdmin
			}
		}
		if err := tx.Model(&member).Update("role", normalizedRole).Error; err != nil {
			return err
		}
		if err := authz.ReplaceAppRoleBinding(tx, member.UserID, appID, normalizedRole, createdBy); err != nil {
			return err
		}
		return tx.Preload("User").First(&member, member.ID).Error
	})
	return member, err
}

func RemoveApplicationMember(db *gorm.DB, appID uint, memberID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if _, err := findApplication(tx, appID); err != nil {
			return err
		}
		var member model.AppMember
		if err := tx.Where("application_id = ? AND id = ?", appID, memberID).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAppMemberNotFound
			}
			return err
		}
		if member.Role == model.AppRoleAdmin {
			count, err := applicationAdminCount(tx, appID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return ErrLastAppAdmin
			}
		}
		if err := tx.Delete(&member).Error; err != nil {
			return err
		}
		return authz.RemoveAppRoleBindings(tx, member.UserID, appID)
	})
}

func findApplication(db *gorm.DB, appID uint) (model.Application, error) {
	var app model.Application
	if err := db.First(&app, appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Application{}, ErrApplicationNotFound
		}
		return model.Application{}, err
	}
	return app, nil
}

func normalizeAppRole(db *gorm.DB, role string) (string, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = model.AppRoleMember
	}
	var count int64
	if err := db.Model(&model.Role{}).
		Where("code = ? AND scope_type = ? AND enabled = ?", role, model.ScopeApp, true).
		Count(&count).Error; err != nil {
		return "", err
	}
	if count == 0 {
		return "", ErrInvalidAppRole
	}
	return role, nil
}

func applicationAdminCount(db *gorm.DB, appID uint) (int64, error) {
	var count int64
	if err := db.Model(&model.AppMember{}).
		Where("application_id = ? AND role = ?", appID, model.AppRoleAdmin).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
