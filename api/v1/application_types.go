package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	// 应用名称（展示用，可中文）
	Name string `json:"name"`

	// 应用标识（K8s 资源用，纯英文）
	// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]*[a-z0-9]$`
	Identifier string `json:"identifier"`

	// 应用描述
	// +optional
	Description string `json:"description,omitempty"`

	// 应用管理员
	// +optional
	Owners []OwnerSpec `json:"owners,omitempty"`
}

type OwnerSpec struct {
	User string `json:"user"`
	Role string `json:"role"`
}

// ApplicationStatus defines the observed state of Application
type ApplicationStatus struct {
	// Phase: Active / Deleting / Error
	Phase string `json:"phase,omitempty"`

	// 环境列表（自动汇总）
	Environments []EnvironmentSummary `json:"environments,omitempty"`

	// 环境数量
	EnvironmentCount int `json:"environmentCount,omitempty"`

	// 条件
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type EnvironmentSummary struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Phase     string `json:"phase"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Identifier",type=string,JSONPath=`.spec.identifier`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Envs",type=integer,JSONPath=`.status.environmentCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Application is the Schema for the applications API
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
