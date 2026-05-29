package model

import (
	"time"

	"gorm.io/gorm"
)

type Application struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	Identifier  string         `gorm:"uniqueIndex;size:50;not null" json:"identifier"`
	Description string         `gorm:"size:500" json:"description"`
	OwnerID     uint           `gorm:"not null" json:"ownerId"`
}
