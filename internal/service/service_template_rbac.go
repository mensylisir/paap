package service

import (
	"encoding/json"

	paapv1 "paap/api/v1"
	"paap/internal/model"

	"gorm.io/gorm"
)

func LoadServiceWorkloadRole(db *gorm.DB, serviceType string) paapv1.RoleSpec {
	tmpl, err := loadServiceTemplateByType(db, serviceType, false)
	if err != nil {
		return noWorkloadRole()
	}
	return ServiceWorkloadRoleFromTemplate(&tmpl)
}

func ServiceWorkloadRoleFromTemplate(svcTmpl *model.ServiceTemplate) paapv1.RoleSpec {
	rules, ok := serviceTemplateManifestRules(svcTmpl, func(manifest *model.PlatformManifest) string {
		return manifest.ToWorkloadRoleJSON()
	})
	if !ok {
		return noWorkloadRole()
	}
	return paapv1.RoleSpec{Rules: rules}
}

func LoadServiceEnvironmentRole(db *gorm.DB, serviceType string) *paapv1.RoleSpec {
	tmpl, err := loadServiceTemplateByType(db, serviceType, false)
	if err != nil {
		return nil
	}
	return ServiceEnvironmentRoleFromTemplate(&tmpl)
}

func ServiceEnvironmentRoleFromTemplate(svcTmpl *model.ServiceTemplate) *paapv1.RoleSpec {
	rules, ok := serviceTemplateManifestRules(svcTmpl, func(manifest *model.PlatformManifest) string {
		return manifest.ToEnvironmentRoleJSON()
	})
	if !ok {
		return nil
	}
	return &paapv1.RoleSpec{Rules: rules}
}

func ServiceToolNamespaceRoleFromTemplate(svcTmpl *model.ServiceTemplate) paapv1.RoleSpec {
	rules, ok := serviceTemplateManifestRules(svcTmpl, func(manifest *model.PlatformManifest) string {
		return manifest.ToToolNamespaceRoleJSON()
	})
	if ok {
		return paapv1.RoleSpec{Rules: rules}
	}

	return defaultSafeToolNamespaceRole()
}

func ServiceClusterRoleFromTemplate(svcTmpl *model.ServiceTemplate) *paapv1.RoleSpec {
	rules, ok := serviceTemplateManifestRules(svcTmpl, func(manifest *model.PlatformManifest) string {
		return manifest.ToClusterRoleJSON()
	})
	if !ok {
		return nil
	}
	return &paapv1.RoleSpec{Rules: rules}
}

func serviceTemplateManifestRules(svcTmpl *model.ServiceTemplate, roleJSON func(*model.PlatformManifest) string) ([]paapv1.PolicyRule, bool) {
	if svcTmpl == nil || svcTmpl.PlatformManifestJSON == "" {
		return nil, false
	}
	var manifest model.PlatformManifest
	if err := json.Unmarshal([]byte(svcTmpl.PlatformManifestJSON), &manifest); err != nil {
		return nil, false
	}
	var rules []paapv1.PolicyRule
	if err := json.Unmarshal([]byte(roleJSON(&manifest)), &rules); err != nil || len(rules) == 0 {
		return nil, false
	}
	return rules, true
}

func defaultSafeToolNamespaceRole() paapv1.RoleSpec {
	return paapv1.RoleSpec{
		Rules: []paapv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "services", "endpoints"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

func noWorkloadRole() paapv1.RoleSpec {
	return paapv1.RoleSpec{Rules: []paapv1.PolicyRule{}}
}
