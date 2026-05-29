package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentSpec defines the desired state of Component
type ComponentSpec struct {
	// 关联的环境（同 namespace 下的 Environment CR 名）
	EnvironmentRef ObjectReference `json:"environmentRef"`

	// 组件名称（展示用，可中文）
	Name string `json:"name"`

	// 组件标识（K8s 资源用，纯英文）
	// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]*[a-z0-9]$`
	Identifier string `json:"identifier"`

	// 组件类型：frontend / backend / custom
	Type string `json:"type"`

	// 谁管理 Deployment：operator / argocd
	// +kubebuilder:validation:Enum=operator;argocd
	// +kubebuilder:default=operator
	ManagedBy string `json:"managedBy,omitempty"`

	// ArgoCD Application 引用（managedBy=argocd 时使用）
	// +optional
	ArgoCDAppRef *ObjectReference `json:"argocdAppRef,omitempty"`

	// 部署配置
	Deployment DeploymentSpec `json:"deployment"`

	// 服务配置
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// Ingress 配置
	// +optional
	Ingress *IngressSpec `json:"ingress,omitempty"`
}

type DeploymentSpec struct {
	// 部署到哪个 namespace
	Namespace string `json:"namespace"`

	// 容器镜像
	Image string `json:"image"`

	// 镜像标签
	Tag string `json:"tag"`

	// 副本数
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`

	// 资源配置
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// 环境变量
	// +optional
	Env []EnvVar `json:"env,omitempty"`
}

type ResourceRequirements struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type EnvVar struct {
	Name      string `json:"name"`
	Value     string `json:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

type EnvVarSource struct {
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

type SecretKeySelector struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type ServiceSpec struct {
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Type       string `json:"type,omitempty"` // ClusterIP / NodePort / LoadBalancer
}

type IngressSpec struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host,omitempty"`
	Path    string `json:"path,omitempty"`
}

// ComponentStatus defines the observed state of Component
type ComponentStatus struct {
	// Phase: Pending / Creating / Running / Scaling / Deleting / Error
	Phase string `json:"phase,omitempty"`

	// Deployment 状态
	Deployment *DeploymentStatus `json:"deployment,omitempty"`

	// Service 状态
	Service *ServiceStatus `json:"service,omitempty"`

	// 条件
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// 观察到的 generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type DeploymentStatus struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	ReadyReplicas  int32  `json:"readyReplicas"`
	Replicas       int32  `json:"replicas"`
	UpdatedReplicas int32 `json:"updatedReplicas"`
}

type ServiceStatus struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	ClusterIP string `json:"clusterIP,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="ManagedBy",type=string,JSONPath=`.spec.managedBy`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Component is the Schema for the components API
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentSpec   `json:"spec,omitempty"`
	Status ComponentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentList contains a list of Component
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
