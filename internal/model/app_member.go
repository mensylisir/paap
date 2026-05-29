package model

import (
	"time"

	"gorm.io/gorm"
)

type AppMember struct {
	ID            uint        `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	ApplicationID uint        `gorm:"not null;index" json:"applicationId"`
	Application   Application `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	UserID        uint        `gorm:"not null;index" json:"userId"`
	User          User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Role          string      `gorm:"size:20;default:member" json:"role"` // admin, member, viewer
}
