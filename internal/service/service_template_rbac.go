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
	if svcTmpl == nil || svcTmpl.WorkloadRolePolicy == "" {
		return noWorkloadRole()
	}
	var rules []paapv1.PolicyRule
	if err := json.Unmarshal([]byte(svcTmpl.WorkloadRolePolicy), &rules); err != nil {
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
	if svcTmpl == nil || svcTmpl.EnvironmentRolePolicy == "" {
		return nil
	}
	var rules []paapv1.PolicyRule
	if err := json.Unmarshal([]byte(svcTmpl.EnvironmentRolePolicy), &rules); err != nil || len(rules) == 0 {
		return nil
	}
	return &paapv1.RoleSpec{Rules: rules}
}

func ServiceToolNamespaceRoleFromTemplate(svcTmpl *model.ServiceTemplate) paapv1.RoleSpec {
	if svcTmpl == nil {
		return defaultSafeToolNamespaceRole()
	}

	if svcTmpl.PlatformManifestJSON != "" {
		var manifest model.PlatformManifest
		if err := json.Unmarshal([]byte(svcTmpl.PlatformManifestJSON), &manifest); err == nil {
			var rules []paapv1.PolicyRule
			if err := json.Unmarshal([]byte(manifest.ToToolNamespaceRoleJSON()), &rules); err == nil && len(rules) > 0 {
				return paapv1.RoleSpec{Rules: rules}
			}
		}
	}

	return defaultSafeToolNamespaceRole()
}

func ServiceClusterRoleFromTemplate(svcTmpl *model.ServiceTemplate) *paapv1.RoleSpec {
	if svcTmpl == nil || svcTmpl.PlatformManifestJSON == "" {
		return nil
	}
	var manifest model.PlatformManifest
	if err := json.Unmarshal([]byte(svcTmpl.PlatformManifestJSON), &manifest); err != nil {
		return nil
	}
	var rules []paapv1.PolicyRule
	if err := json.Unmarshal([]byte(manifest.ToClusterRoleJSON()), &rules); err != nil || len(rules) == 0 {
		return nil
	}
	return &paapv1.RoleSpec{Rules: rules}
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
