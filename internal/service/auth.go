package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"paap/internal/authz"
	"paap/internal/middleware"
	"paap/internal/model"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type RegisterUserInput struct {
	Username string
	Password string
	Email    string
}

type AuthUser struct {
	ID       uint     `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email,omitempty"`
	Roles    []string `json:"roles"`
}

type LoginResult struct {
	Token string `json:"token"`
	AuthUser
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func RegisterUser(db *gorm.DB, input RegisterUserInput) (AuthUser, error) {
	hash, err := HashPassword(input.Password)
	if err != nil {
		return AuthUser{}, err
	}

	user := model.User{
		Username: input.Username,
		Password: hash,
		Email:    input.Email,
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return authz.BindRole(tx, user.ID, model.RoleUser, authz.SystemScope(), user.ID)
	}); err != nil {
		return AuthUser{}, err
	}
	return authUserFromModel(db, user)
}

func LoginUser(db *gorm.DB, username, password string) (LoginResult, error) {
	var user model.User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	if !CheckPasswordHash(password, user.Password) {
		return LoginResult{}, ErrInvalidCredentials
	}
	roles, err := authz.SystemRoleCodes(db, user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	token, err := middleware.GenerateToken(user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		Token: token,
		AuthUser: AuthUser{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Roles:    roles,
		},
	}, nil
}

func CurrentUser(db *gorm.DB, userID uint) (AuthUser, error) {
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		return AuthUser{}, ErrUserNotFound
	}
	return authUserFromModel(db, user)
}

func UpsertKeycloakUser(db *gorm.DB, info map[string]interface{}, roles []string) (model.User, []string, error) {
	username := firstUserInfoString(info, "preferred_username", "email", "sub")
	if username == "" {
		return model.User{}, nil, errors.New("keycloak userinfo is missing username")
	}
	email := firstUserInfoString(info, "email")
	if email == "" {
		email = username + "@keycloak.local"
	}

	var user model.User
	err := db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("username = ?", username).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			passwordHash, err := randomInitialUserPasswordHash()
			if err != nil {
				return err
			}
			user = model.User{Username: username, Email: email, Password: passwordHash}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if email != "" && user.Email != email {
			if err := tx.Model(&user).Update("email", email).Error; err != nil {
				return err
			}
			user.Email = email
		}
		_, err = authz.ReplaceSystemRoleBindings(tx, user.ID, roles, user.ID)
		return err
	})
	if err != nil {
		return model.User{}, nil, err
	}
	return user, roles, nil
}

func authUserFromModel(db *gorm.DB, user model.User) (AuthUser, error) {
	roles, err := authz.SystemRoleCodes(db, user.ID)
	if err != nil {
		return AuthUser{}, err
	}
	return AuthUser{ID: user.ID, Username: user.Username, Email: user.Email, Roles: roles}, nil
}

func firstUserInfoString(info map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := info[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func randomInitialUserPasswordHash() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return HashPassword(hex.EncodeToString(raw[:]))
}
