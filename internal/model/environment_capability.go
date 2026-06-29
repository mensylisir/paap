package model

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
)

const (
	CapabilitySourceManaged  = "managed"
	CapabilitySourceShared   = "shared"
	CapabilitySourceExternal = "external"
	CapabilitySourceDeferred = "deferred"
)

type EnvironmentCapability struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`

	EnvironmentID uint        `gorm:"not null;uniqueIndex:idx_environment_capability_key" json:"environmentId"`
	Environment   Environment `gorm:"foreignKey:EnvironmentID" json:"environment,omitempty"`

	Capability    string `gorm:"size:40;not null;index" json:"capability"`
	CapabilityKey string `gorm:"size:100;not null;uniqueIndex:idx_environment_capability_key" json:"capabilityKey"`
	Source        string `gorm:"size:20;not null;default:managed" json:"source"`
	Provider      string `gorm:"size:50" json:"provider"`

	ServiceType  string               `gorm:"size:30" json:"serviceType,omitempty"`
	RefServiceID *uint                `gorm:"index" json:"refServiceId,omitempty"`
	RefService   *ServiceInstallation `gorm:"foreignKey:RefServiceID" json:"refService,omitempty"`

	ExternalEndpoint      string `gorm:"size:500" json:"externalEndpoint,omitempty"`
	CredentialSecretRef   string `gorm:"size:200" json:"credentialSecretRef,omitempty"`
	TLSInsecureSkipVerify bool   `gorm:"default:false" json:"tlsInsecureSkipVerify,omitempty"`

	ValidationStatus  string `gorm:"size:20;default:pending" json:"validationStatus"`
	ValidationMessage string `gorm:"type:text" json:"validationMessage,omitempty"`
	CreatedBy         uint   `gorm:"index" json:"createdBy,omitempty"`
}

func (c *EnvironmentCapability) BeforeSave(tx *gorm.DB) error {
	if strings.TrimSpace(c.CapabilityKey) != "" {
		c.CapabilityKey = normalizeEnvironmentCapabilityKey(c.CapabilityKey, c.Capability)
		return nil
	}
	parts := []string{c.Capability, c.Source}
	if c.RefServiceID != nil && *c.RefServiceID != 0 {
		parts = append(parts, fmt.Sprintf("%d", *c.RefServiceID))
	} else {
		switch c.Source {
		case CapabilitySourceExternal:
			parts = append(parts, firstEnvironmentCapabilityKeyPart(c.Provider, c.ServiceType, c.ExternalEndpoint, "draft"))
		case CapabilitySourceDeferred:
			parts = append(parts, firstEnvironmentCapabilityKeyPart(c.Provider, c.ServiceType, "pending"))
		}
	}
	c.CapabilityKey = normalizeEnvironmentCapabilityKey(strings.Join(parts, "-"), c.Capability)
	return nil
}

func firstEnvironmentCapabilityKeyPart(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "capability"
}

func normalizeEnvironmentCapabilityKey(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	clean := strings.Trim(b.String(), "-")
	if clean == "" {
		clean = strings.TrimSpace(fallback)
	}
	if clean == "" {
		clean = "capability"
	}
	if len(clean) > 100 {
		clean = strings.Trim(clean[:100], "-")
	}
	return clean
}
