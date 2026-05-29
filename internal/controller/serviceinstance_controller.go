package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	paapv1 "paap/api/v1"
)

const svcFinalizer = "paap.io/serviceinstance-finalizer"

// ServiceInstanceReconciler reconciles a ServiceInstance object
type ServiceInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=paap.io,resources=serviceinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=paap.io,resources=serviceinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=paap.io,resources=serviceinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=paap.io,resources=environments,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets,verbs=get;list;watch;create;update;delete

// paapLabels 返回标准的 PAAP 标签
func paapLabels(appIdentifier, envIdentifier, toolType string) map[string]string {
	return map[string]string{
		"paap.io/app":  appIdentifier,
		"paap.io/env":  envIdentifier,
		"paap.io/tool": toolType,
		"paap.io/managed-by": "paap-operator",
	}
}

// paapAnnotations 返回标准的 PAAP 注解
func paapAnnotations(toolNamespace, appIdentifier, envIdentifier, toolType string) map[string]string {
	return map[string]string{
		"paap.io/tool-namespace": toolNamespace,
		"paap.io/app":            appIdentifier,
		"paap.io/env":            envIdentifier,
		"paap.io/tool":           toolType,
	}
}

func (r *ServiceInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	svc := &paapv1.ServiceInstance{}
	if err := r.Get(ctx, req.NamespacedName, svc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	appIdentifier := svc.Labels["paap.io/app"]
	envIdentifier := svc.Labels["paap.io/env"]
	toolType := svc.Spec.Type
	toolNS := svc.Spec.ToolNamespace

	// 处理删除
	if !svc.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, svc)
	}

	// 添加 Finalizer
	if !controllerutil.ContainsFinalizer(svc, svcFinalizer) {
		controllerutil.AddFinalizer(svc, svcFinalizer)
		if err := r.Update(ctx, svc); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 获取关联的 Environment
	env := &paapv1.Environment{}
	envKey := types.NamespacedName{
		Name:      svc.Spec.EnvironmentRef.Name,
		Namespace: svc.Namespace,
	}
	if err := r.Get(ctx, envKey, env); err != nil {
		if apierrors.IsNotFound(err) {
			svc.Status.Phase = "Error"
			r.Status().Update(ctx, svc)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// 检查 Environment 是否就绪
	if env.Status.Phase != "Running" {
		logger.Info("Environment not ready, waiting", "envPhase", env.Status.Phase)
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	// Step 1: 创建工具独占 namespace
	if err := r.ensureToolNamespace(ctx, toolNS, appIdentifier, envIdentifier, toolType); err != nil {
		logger.Error(err, "failed to ensure tool namespace", "namespace", toolNS)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// Step 2: 在工具 ns 内创建 SA
	if err := r.ensureServiceAccount(ctx, svc, toolNS, appIdentifier, envIdentifier, toolType); err != nil {
		logger.Error(err, "failed to ensure SA")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// Step 3: 在工具 ns 内创建 deploymentRole（工具自身需要的权限）
	if svc.Spec.DeploymentRole != nil {
		if err := r.ensureRole(ctx, svc, toolNS, "deployment", svc.Spec.DeploymentRole, appIdentifier, envIdentifier, toolType); err != nil {
			logger.Error(err, "failed to ensure deploymentRole")
		}
		r.ensureRoleBinding(ctx, svc, toolNS, "deployment", toolNS, appIdentifier, envIdentifier, toolType)
	}

	// Step 4: 在工具 ns 内创建自管理 Role（工具对自己 ns 的完整权限）
	selfRole := &paapv1.RoleSpec{
		Rules: []paapv1.PolicyRule{
			{
				APIGroups: []string{"", "apps", "batch", "networking.k8s.io", "autoscaling"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}
	if err := r.ensureRole(ctx, svc, toolNS, "self", selfRole, appIdentifier, envIdentifier, toolType); err != nil {
		logger.Error(err, "failed to ensure self role in tool namespace")
	}
	r.ensureRoleBinding(ctx, svc, toolNS, "self", toolNS, appIdentifier, envIdentifier, toolType)

	// Step 5: 发现所有负载 namespace（通过标签查询），创建 workloadRole
	workloadNSList := r.discoverWorkloadNamespaces(ctx, appIdentifier, envIdentifier)
	rbacStatuses := make([]paapv1.RBACNamespaceStatus, 0, len(workloadNSList)+1)
	rbacStatuses = append(rbacStatuses, paapv1.RBACNamespaceStatus{
		Namespace:          toolNS,
		RoleCreated:        true,
		RoleBindingCreated: true,
	})

	for _, nsName := range workloadNSList {
		if nsName == toolNS {
			continue // 跳过工具自己的 ns
		}
		if err := r.ensureRole(ctx, svc, nsName, "workload", &svc.Spec.WorkloadRole, appIdentifier, envIdentifier, toolType); err != nil {
			logger.Error(err, "failed to ensure workload Role", "namespace", nsName)
			rbacStatuses = append(rbacStatuses, paapv1.RBACNamespaceStatus{
				Namespace: nsName, RoleCreated: false, RoleBindingCreated: false,
			})
			continue
		}
		if err := r.ensureRoleBinding(ctx, svc, nsName, "workload", toolNS, appIdentifier, envIdentifier, toolType); err != nil {
			logger.Error(err, "failed to ensure workload RoleBinding", "namespace", nsName)
			rbacStatuses = append(rbacStatuses, paapv1.RBACNamespaceStatus{
				Namespace: nsName, RoleCreated: true, RoleBindingCreated: false,
			})
			continue
		}
		rbacStatuses = append(rbacStatuses, paapv1.RBACNamespaceStatus{
			Namespace: nsName, RoleCreated: true, RoleBindingCreated: true,
		})
	}

	// Step 6: 部署工具组件（从 ConfigMap 读取渲染后的 manifests）
	if err := r.ensureToolComponents(ctx, svc); err != nil {
		logger.Error(err, "failed to ensure tool components")
	}

	// 更新 status
	svc.Status.Phase = "Running"
	svc.Status.ServiceAccount = &paapv1.ServiceAccountStatus{
		Name:      svc.Spec.ServiceAccount.Name,
		Namespace: toolNS,
		Created:   true,
	}
	svc.Status.RBACNamespaces = rbacStatuses
	svc.Status.ObservedGeneration = svc.Generation

	if err := r.Status().Update(ctx, svc); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// ensureToolNamespace 创建工具独占的 namespace，打全量标签和注解
func (r *ServiceInstanceReconciler) ensureToolNamespace(ctx context.Context, nsName, appIdentifier, envIdentifier, toolType string) error {
	ns := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: nsName}, ns)
	if err == nil {
		return nil // 已存在
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	labels := paapLabels(appIdentifier, envIdentifier, toolType)
	labels["paap.io/role"] = "tool" // 标记为工具 namespace

	annotations := paapAnnotations(nsName, appIdentifier, envIdentifier, toolType)

	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nsName,
			Labels:      labels,
			Annotations: annotations,
		},
	}
	return r.Create(ctx, ns)
}

// discoverWorkloadNamespaces 通过标签发现环境的所有负载 namespace
func (r *ServiceInstanceReconciler) discoverWorkloadNamespaces(ctx context.Context, appIdentifier, envIdentifier string) []string {
	nsList := &corev1.NamespaceList{}
	labels := client.MatchingLabels{
		"paap.io/app":  appIdentifier,
		"paap.io/env":  envIdentifier,
		"paap.io/role": "workload",
	}
	if err := r.List(ctx, nsList, labels); err != nil {
		return nil
	}
	result := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		result = append(result, ns.Name)
	}
	return result
}

func (r *ServiceInstanceReconciler) handleDeletion(ctx context.Context, svc *paapv1.ServiceInstance) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(svc, svcFinalizer) {
		return ctrl.Result{}, nil
	}

	toolNS := svc.Spec.ToolNamespace
	appIdentifier := svc.Labels["paap.io/app"]
	envIdentifier := svc.Labels["paap.io/env"]
	toolType := svc.Spec.Type

	// 删除工具 namespace（级联删除所有资源）
	if toolNS != "" {
		ns := &corev1.Namespace{}
		if err := r.Get(ctx, types.NamespacedName{Name: toolNS}, ns); err == nil {
			logger.Info("deleting tool namespace", "namespace", toolNS)
			if err := r.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: 2 * time.Second}, err
			}
		}
	}

	// 清理负载 namespace 中的 Role 和 RoleBinding
	workloadNSList := r.discoverWorkloadNamespaces(ctx, appIdentifier, envIdentifier)
	roleName := fmt.Sprintf("%s-%s-%s-workload-manager", appIdentifier, envIdentifier, toolType)
	for _, nsName := range workloadNSList {
		role := &rbacv1.Role{}
		if err := r.Get(ctx, types.NamespacedName{Name: roleName, Namespace: nsName}, role); err == nil {
			r.Delete(ctx, role)
		}
		rb := &rbacv1.RoleBinding{}
		if err := r.Get(ctx, types.NamespacedName{Name: roleName, Namespace: nsName}, rb); err == nil {
			r.Delete(ctx, rb)
		}
	}

	// 移除 Finalizer
	controllerutil.RemoveFinalizer(svc, svcFinalizer)
	if err := r.Update(ctx, svc); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("ServiceInstance deleted", "name", svc.Name, "type", toolType)
	return ctrl.Result{}, nil
}

func (r *ServiceInstanceReconciler) ensureServiceAccount(ctx context.Context, svc *paapv1.ServiceInstance, nsName, appIdentifier, envIdentifier, toolType string) error {
	sa := &corev1.ServiceAccount{}
	saKey := types.NamespacedName{
		Name:      svc.Spec.ServiceAccount.Name,
		Namespace: nsName,
	}
	if err := r.Get(ctx, saKey, sa); err == nil {
		return nil
	}

	labels := paapLabels(appIdentifier, envIdentifier, toolType)
	annotations := paapAnnotations(svc.Spec.ToolNamespace, appIdentifier, envIdentifier, toolType)

	sa = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svc.Spec.ServiceAccount.Name,
			Namespace:   nsName,
			Labels:      labels,
			Annotations: annotations,
		},
	}
	return r.Create(ctx, sa)
}

func (r *ServiceInstanceReconciler) ensureRole(ctx context.Context, svc *paapv1.ServiceInstance, nsName, roleType string, roleSpec *paapv1.RoleSpec, appIdentifier, envIdentifier, toolType string) error {
	roleName := fmt.Sprintf("%s-%s-%s-%s-manager", appIdentifier, envIdentifier, toolType, roleType)
	role := &rbacv1.Role{}
	roleKey := types.NamespacedName{Name: roleName, Namespace: nsName}
	if err := r.Get(ctx, roleKey, role); err == nil {
		return nil // 已存在
	}

	logger := log.FromContext(ctx)
	logger.Info("creating Role", "name", roleName, "namespace", nsName, "type", roleType)

	rules := make([]rbacv1.PolicyRule, 0, len(roleSpec.Rules))
	for _, rule := range roleSpec.Rules {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: rule.APIGroups,
			Resources: rule.Resources,
			Verbs:     rule.Verbs,
		})
	}

	labels := paapLabels(appIdentifier, envIdentifier, toolType)
	annotations := paapAnnotations(svc.Spec.ToolNamespace, appIdentifier, envIdentifier, toolType)

	role = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:        roleName,
			Namespace:   nsName,
			Labels:      labels,
			Annotations: annotations,
		},
		Rules: rules,
	}
	return r.Create(ctx, role)
}

func (r *ServiceInstanceReconciler) ensureRoleBinding(ctx context.Context, svc *paapv1.ServiceInstance, nsName, roleType, saNamespace, appIdentifier, envIdentifier, toolType string) error {
	roleName := fmt.Sprintf("%s-%s-%s-%s-manager", appIdentifier, envIdentifier, toolType, roleType)
	rbName := roleName
	rb := &rbacv1.RoleBinding{}
	rbKey := types.NamespacedName{Name: rbName, Namespace: nsName}
	if err := r.Get(ctx, rbKey, rb); err == nil {
		return nil
	}

	labels := paapLabels(appIdentifier, envIdentifier, toolType)
	annotations := paapAnnotations(svc.Spec.ToolNamespace, appIdentifier, envIdentifier, toolType)

	rb = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rbName,
			Namespace:   nsName,
			Labels:      labels,
			Annotations: annotations,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      svc.Spec.ServiceAccount.Name,
				Namespace: saNamespace,
			},
		},
	}
	return r.Create(ctx, rb)
}

// ensureToolComponents reads rendered manifests from ConfigMap and applies them to the tool namespace
func (r *ServiceInstanceReconciler) ensureToolComponents(ctx context.Context, svc *paapv1.ServiceInstance) error {
	if svc.Spec.ManifestsRef == nil {
		return nil
	}

	logger := log.FromContext(ctx)

	// 读取 ConfigMap
	cm := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{
		Name:      svc.Spec.ManifestsRef.Name,
		Namespace: svc.Spec.ManifestsRef.Namespace,
	}
	if err := r.Get(ctx, cmKey, cm); err != nil {
		logger.Error(err, "failed to read manifests ConfigMap", "configmap", cmKey)
		return err
	}

	manifestsYaml, ok := cm.Data["manifests.yaml"]
	if !ok || manifestsYaml == "" {
		logger.Info("no manifests.yaml in ConfigMap", "configmap", cmKey)
		return nil
	}

	toolNS := svc.Spec.ToolNamespace
	appIdentifier := svc.Labels["paap.io/app"]
	envIdentifier := svc.Labels["paap.io/env"]
	toolType := svc.Spec.Type

	// 解析多文档 YAML
	docs := splitYAML(manifestsYaml)
	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		// 解析 YAML 到 unstructured object
		obj := &unstructured.Unstructured{}
		// 先用 sigs.k8s.io/yaml 转 JSON，再用 UnstructuredJSONScheme 解析
		jsonData, err := yaml.YAMLToJSON([]byte(doc))
		if err != nil {
			logger.Error(err, "failed to convert YAML to JSON")
			continue
		}
		if _, _, err := unstructured.UnstructuredJSONScheme.Decode(jsonData, nil, obj); err != nil {
			logger.Error(err, "failed to decode JSON to unstructured")
			continue
		}

		// 强制设置 namespace
		obj.SetNamespace(toolNS)

		// 添加标准标签
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["paap.io/app"] = appIdentifier
		labels["paap.io/env"] = envIdentifier
		labels["paap.io/tool"] = toolType
		labels["paap.io/managed-by"] = "paap-operator"
		obj.SetLabels(labels)

		// 添加标准注解
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations["paap.io/tool-namespace"] = toolNS
		annotations["paap.io/app"] = appIdentifier
		annotations["paap.io/env"] = envIdentifier
		annotations["paap.io/tool"] = toolType
		obj.SetAnnotations(annotations)

		// 尝试 Get，不存在则 Create，已存在则 Update
		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(obj.GroupVersionKind())
		err = r.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)
		if err != nil {
			if apierrors.IsNotFound(err) {
				if err := r.Create(ctx, obj); err != nil {
					logger.Error(err, "failed to create manifest", "kind", obj.GetKind(), "name", obj.GetName())
					continue
				}
				logger.Info("created manifest", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", toolNS)
			} else {
				logger.Error(err, "failed to get manifest", "kind", obj.GetKind(), "name", obj.GetName())
			}
			continue
		}

		// 已存在，更新
		obj.SetResourceVersion(existing.GetResourceVersion())
		if err := r.Update(ctx, obj); err != nil {
			logger.Error(err, "failed to update manifest", "kind", obj.GetKind(), "name", obj.GetName())
			continue
		}
		logger.Info("updated manifest", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", toolNS)
	}

	return nil
}

// splitYAML splits a multi-document YAML string into individual documents
func splitYAML(yamlStr string) []string {
	// 清理 YAML 内容
	yamlStr = strings.TrimSpace(yamlStr)

	// 按 --- 分割
	docs := strings.Split(yamlStr, "\n---")

	result := make([]string, 0, len(docs))
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc != "" && doc != "---" {
			result = append(result, doc)
		}
	}
	return result
}

func (r *ServiceInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paapv1.ServiceInstance{}).
		Complete(r)
}
