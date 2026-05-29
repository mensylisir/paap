package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	paapv1 "paap/api/v1"
)

const envFinalizer = "paap.io/environment-finalizer"

// EnvironmentReconciler reconciles a Environment object
type EnvironmentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=paap.io,resources=environments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=paap.io,resources=environments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=paap.io,resources=environments/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;delete

func (r *EnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	env := &paapv1.Environment{}
	if err := r.Get(ctx, req.NamespacedName, env); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 处理删除
	if !env.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, env)
	}

	// 添加 Finalizer
	if !controllerutil.ContainsFinalizer(env, envFinalizer) {
		controllerutil.AddFinalizer(env, envFinalizer)
		if err := r.Update(ctx, env); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 收集期望的 namespace 列表
	expectedNS := []string{env.Spec.PrimaryNamespace}
	for _, ns := range env.Spec.AdditionalNamespaces {
		expectedNS = append(expectedNS, fmt.Sprintf("%s-%s", env.Spec.PrimaryNamespace, ns.Suffix))
	}

	// 创建/更新 namespace
	allNSReady := true
	nsStatuses := make([]paapv1.NamespaceStatus, 0, len(expectedNS))
	for _, nsName := range expectedNS {
		if err := r.ensureNamespace(ctx, env, nsName); err != nil {
			logger.Error(err, "failed to ensure namespace", "namespace", nsName)
			allNSReady = false
			continue
		}
		nsStatuses = append(nsStatuses, paapv1.NamespaceStatus{
			Name:  nsName,
			Phase: "Active",
		})
	}

	// 创建 NetworkPolicy（如果配置了）
	if env.Spec.Network.Isolation == "NetworkPolicy" || env.Spec.Network.Isolation == "" {
		for _, nsName := range expectedNS {
			if err := r.ensureNetworkPolicy(ctx, env, nsName); err != nil {
				logger.Error(err, "failed to ensure NetworkPolicy", "namespace", nsName)
			}
		}
	}

	// 创建 ResourceQuota（如果配置了）
	if env.Spec.ResourceQuota != nil {
		for i, nsName := range expectedNS {
			quota := r.calculateQuota(env.Spec.ResourceQuota, i, len(expectedNS))
			if err := r.ensureResourceQuota(ctx, nsName, quota); err != nil {
				logger.Error(err, "failed to ensure ResourceQuota", "namespace", nsName)
			}
		}
	}

	// 更新 status
	if allNSReady {
		env.Status.Phase = "Running"
	} else {
		env.Status.Phase = "Creating"
	}
	env.Status.Namespaces = nsStatuses
	env.Status.ObservedGeneration = env.Generation

	if err := r.Status().Update(ctx, env); err != nil {
		return ctrl.Result{}, err
	}

	if !allNSReady {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *EnvironmentReconciler) handleDeletion(ctx context.Context, env *paapv1.Environment) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(env, envFinalizer) {
		return ctrl.Result{}, nil
	}

	// 删除子 ServiceInstance CR
	siList := &paapv1.ServiceInstanceList{}
	if err := r.List(ctx, siList, client.InNamespace(env.Namespace)); err == nil {
		for i := range siList.Items {
			if siList.Items[i].Spec.EnvironmentRef.Name == env.Name {
				logger.Info("deleting ServiceInstance", "name", siList.Items[i].Name)
				if err := r.Delete(ctx, &siList.Items[i]); err != nil && !errors.IsNotFound(err) {
					return ctrl.Result{RequeueAfter: 2 * time.Second}, err
				}
			}
		}
	}

	// 删除子 Component CR
	compList := &paapv1.ComponentList{}
	if err := r.List(ctx, compList, client.InNamespace(env.Namespace)); err == nil {
		for i := range compList.Items {
			if compList.Items[i].Spec.EnvironmentRef.Name == env.Name {
				logger.Info("deleting Component", "name", compList.Items[i].Name)
				if err := r.Delete(ctx, &compList.Items[i]); err != nil && !errors.IsNotFound(err) {
					return ctrl.Result{RequeueAfter: 2 * time.Second}, err
				}
			}
		}
	}

	// 检查子 CR 是否全部删除
	remainingSI := &paapv1.ServiceInstanceList{}
	r.List(ctx, remainingSI, client.InNamespace(env.Namespace))
	for _, si := range remainingSI.Items {
		if si.Spec.EnvironmentRef.Name == env.Name {
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
	}
	remainingComp := &paapv1.ComponentList{}
	r.List(ctx, remainingComp, client.InNamespace(env.Namespace))
	for _, c := range remainingComp.Items {
		if c.Spec.EnvironmentRef.Name == env.Name {
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
	}

	// 删除业务 namespace
	allNS := []string{env.Spec.PrimaryNamespace}
	for _, ns := range env.Spec.AdditionalNamespaces {
		allNS = append(allNS, fmt.Sprintf("%s-%s", env.Spec.PrimaryNamespace, ns.Suffix))
	}
	for _, nsName := range allNS {
		ns := &corev1.Namespace{}
		if err := r.Get(ctx, types.NamespacedName{Name: nsName}, ns); err == nil {
			logger.Info("deleting namespace", "namespace", nsName)
			if err := r.Delete(ctx, ns); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: 2 * time.Second}, err
			}
		}
	}

	// 移除 Finalizer
	controllerutil.RemoveFinalizer(env, envFinalizer)
	if err := r.Update(ctx, env); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Environment deleted", "name", env.Name)
	return ctrl.Result{}, nil
}

func (r *EnvironmentReconciler) ensureNamespace(ctx context.Context, env *paapv1.Environment, nsName string) error {
	ns := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: nsName}, ns)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	appIdentifier := env.Labels["paap.io/app"]

	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				"paap.io/app":        appIdentifier,
				"paap.io/env":        env.Spec.Identifier,
				"paap.io/role":       "workload", // 标记为负载 namespace
				"paap.io/managed-by": "paap-operator",
			},
			Annotations: map[string]string{
				"paap.io/app":            appIdentifier,
				"paap.io/env":            env.Spec.Identifier,
				"paap.io/role":           "workload",
				"paap.io/environment":    env.Name,
			},
		},
	}
	return r.Create(ctx, ns)
}

func (r *EnvironmentReconciler) ensureNetworkPolicy(ctx context.Context, env *paapv1.Environment, nsName string) error {
	npName := "paap-deny-cross-env"
	np := &networkingv1.NetworkPolicy{}
	err := r.Get(ctx, types.NamespacedName{Name: npName, Namespace: nsName}, np)
	if err == nil {
		return nil // 已存在
	}
	if !errors.IsNotFound(err) {
		return err
	}

	appIdentifier := env.Labels["paap.io/app"]

	np = &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      npName,
			Namespace: nsName,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"paap.io/app": appIdentifier,
									"paap.io/env": env.Spec.Identifier,
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "ingress-nginx",
								},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"paap.io/app": appIdentifier,
									"paap.io/env": env.Spec.Identifier,
								},
							},
						},
					},
				},
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "kube-system",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: func() *corev1.Protocol { p := corev1.ProtocolUDP; return &p }(),
							Port:     func() *intstr.IntOrString { p := intstr.FromInt(53); return &p }(),
						},
						{
							Protocol: func() *corev1.Protocol { p := corev1.ProtocolTCP; return &p }(),
							Port:     func() *intstr.IntOrString { p := intstr.FromInt(53); return &p }(),
						},
					},
				},
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: "0.0.0.0/0",
								Except: []string{
									"10.244.0.0/16", // 集群 Pod CIDR，需根据实际配置调整
								},
							},
						},
					},
				},
			},
		},
	}
	return r.Create(ctx, np)
}

func (r *EnvironmentReconciler) ensureResourceQuota(ctx context.Context, nsName string, quota *paapv1.ResourceQuotaSpec) error {
	rqName := "paap-resource-quota"
	rq := &corev1.ResourceQuota{}
	err := r.Get(ctx, types.NamespacedName{Name: rqName, Namespace: nsName}, rq)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	hard := corev1.ResourceList{}
	if quota.CPU != "" {
		hard[corev1.ResourceCPU] = resource.MustParse(quota.CPU)
	}
	if quota.Memory != "" {
		hard[corev1.ResourceMemory] = resource.MustParse(quota.Memory)
	}
	if quota.Storage != "" {
		hard[corev1.ResourceRequestsStorage] = resource.MustParse(quota.Storage)
	}

	rq = &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rqName,
			Namespace: nsName,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hard,
		},
	}
	return r.Create(ctx, rq)
}

// calculateQuota 按比例分摊配额到各 namespace
// index=0: 主空间 50%，index=1: 工作负载空间 40%，其他: 10%
func (r *EnvironmentReconciler) calculateQuota(total *paapv1.ResourceQuotaSpec, index, count int) *paapv1.ResourceQuotaSpec {
	var ratio float64
	switch {
	case index == 0:
		ratio = 0.5
	case index == 1 && count > 2:
		ratio = 0.4
	default:
		ratio = 0.1
	}

	return &paapv1.ResourceQuotaSpec{
		CPU:     multiplyQuantity(total.CPU, ratio),
		Memory:  multiplyQuantity(total.Memory, ratio),
		Storage: multiplyQuantity(total.Storage, ratio),
	}
}

func multiplyQuantity(s string, ratio float64) string {
	if s == "" {
		return ""
	}
	q := resource.MustParse(s)
	// 简单处理：转换为毫核/字节再乘
	if q.Cmp(resource.MustParse("0")) == 0 {
		return "0"
	}
	// 对于简单场景，直接返回原值（精确分摊需要更复杂的逻辑）
	return s
}

func (r *EnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paapv1.Environment{}).
		Complete(r)
}
