package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentSpec defines the desired state of Environment
type EnvironmentSpec struct {
	// 环境名称（展示用，可中文）
	Name string `json:"name"`

	// 环境标识（K8s 资源用，纯英文）
	// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]*[a-z0-9]$`
	Identifier string `json:"identifier"`

	// 主 namespace（自动创建）
	PrimaryNamespace string `json:"primaryNamespace"`

	// 附加 namespace（自动创建）
	// +optional
	AdditionalNamespaces []AdditionalNamespace `json:"additionalNamespaces,omitempty"`

	// 网络配置
	// +optional
	Network NetworkSpec `json:"network,omitempty"`

	// 资源配额（总配额，分摊到各 namespace）
	// +optional
	ResourceQuota *ResourceQuotaSpec `json:"resourceQuota,omitempty"`
}

type AdditionalNamespace struct {
	// 后缀，生成 {app}-{env}-{suffix}
	Suffix string `json:"suffix"`

	// 用途：workload / database / cache
	Purpose string `json:"purpose"`
}

type NetworkSpec struct {
	// 隔离策略：NetworkPolicy / None
	Isolation string `json:"isolation,omitempty"`

	// IP Pool 配置（可选）
	// +optional
	IPPool *IPPoolSpec `json:"ipPool,omitempty"`
}

type IPPoolSpec struct {
	Enabled  bool   `json:"enabled"`
	CIDR     string `json:"cidr,omitempty"`
	Provider string `json:"provider,omitempty"` // calico / metallb / none
}

type ResourceQuotaSpec struct {
	CPU     string `json:"cpu,omitempty"`
	Memory  string `json:"memory,omitempty"`
	Storage string `json:"storage,omitempty"`
}

// EnvironmentStatus defines the observed state of Environment
type EnvironmentStatus struct {
	// Phase: Pending / Creating / Running / Deleting / Error
	Phase string `json:"phase,omitempty"`

	// 实际创建的 namespace 列表
	Namespaces []NamespaceStatus `json:"namespaces,omitempty"`

	// 条件
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// 观察到的 generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type NamespaceStatus struct {
	Name  string `json:"name"`
	Phase string `json:"phase"` // Active / Terminating
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Identifier",type=string,JSONPath=`.spec.identifier`
// +kubebuilder:printcolumn:name="PrimaryNS",type=string,JSONPath=`.spec.primaryNamespace`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Environment is the Schema for the environments API
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvironmentList contains a list of Environment
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
