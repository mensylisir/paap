package controller

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	paapv1 "paap/api/v1"
	paaphelm "paap/internal/helm"
	"paap/internal/k8s"
	"paap/internal/model"
	svcservice "paap/internal/service"
)

const svcFinalizer = "paap.io/serviceinstance-finalizer"

// ServiceInstanceReconciler reconciles a ServiceInstance object
type ServiceInstanceReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HelmClient helmInstaller
}

type helmInstaller interface {
	UpgradeInstallWithMetadata(releaseName, namespace, chartPath string, values map[string]interface{}, metadata paaphelm.ResourceMetadata) error
}

// +kubebuilder:rbac:groups=paap.io,resources=serviceinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=paap.io,resources=serviceinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=paap.io,resources=serviceinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=paap.io,resources=environments,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=alertmanagerconfigs;alertmanagers;podmonitors;probes;prometheusagents;prometheuses;prometheusrules;scrapeconfigs;servicemonitors;thanosrulers,verbs=get;list;watch;create;update;delete

type serviceMetadata struct {
	AppIdentifier string
	EnvIdentifier string
	ServiceType   string
	Tool          string
	Category      string
	ResourceRole  string
	ToolNamespace string
}

func metadataFromServiceInstance(svc *paapv1.ServiceInstance) serviceMetadata {
	md := serviceMetadata{
		AppIdentifier: valueFromMaps("paap.io/app", svc.Labels, svc.Annotations),
		EnvIdentifier: valueFromMaps("paap.io/env", svc.Labels, svc.Annotations),
		ServiceType:   strings.TrimSpace(svc.Spec.Type),
		Tool:          valueFromMaps("paap.io/tool", svc.Labels, svc.Annotations),
		Category:      valueFromMaps("paap.io/category", svc.Labels, svc.Annotations),
		ResourceRole:  valueFromMaps("paap.io/resource-role", svc.Labels, svc.Annotations),
		ToolNamespace: strings.TrimSpace(svc.Spec.ToolNamespace),
	}
	if md.ServiceType == "" {
		md.ServiceType = valueFromMaps("paap.io/service-type", svc.Labels, svc.Annotations)
	}
	if md.Tool == "" {
		md.Tool = md.ServiceType
	}
	if md.Category == "" {
		md.Category = "tool"
	}
	if md.ResourceRole == "" {
		md.ResourceRole = "tool"
	}
	if md.ToolNamespace == "" {
		md.ToolNamespace = valueFromMaps("paap.io/tool-namespace", svc.Annotations, nil)
	}
	return md
}

func expectedConcreteServiceInstanceName(svc *paapv1.ServiceInstance, md serviceMetadata) string {
	toolIdentity := valueFromMaps("paap.io/tool", svc.Labels, svc.Annotations)
	if toolIdentity == "" {
		return ""
	}
	if strings.EqualFold(toolIdentity, md.ServiceType) && svc.Spec.Helm != nil {
		toolIdentity = firstNonEmptyString(
			serviceIdentityFromRuntimeName(md.EnvIdentifier, svc.Spec.Helm.ReleaseName),
			serviceIdentityFromRuntimeName(md.EnvIdentifier, svc.Spec.Helm.Namespace),
			toolIdentity,
		)
	}
	if md.EnvIdentifier == "" || toolIdentity == "" {
		return ""
	}
	return dnsLabelPart(md.EnvIdentifier + "-" + toolIdentity)
}

func serviceIdentityFromRuntimeName(envIdentifier, runtimeName string) string {
	runtimeName = dnsLabelPart(runtimeName)
	envIdentifier = dnsLabelPart(envIdentifier)
	if runtimeName == "" || envIdentifier == "" {
		return ""
	}
	suffix := "-" + envIdentifier + "-"
	if idx := strings.Index(runtimeName, suffix); idx >= 0 {
		return strings.Trim(runtimeName[idx+len(suffix):], "-")
	}
	if strings.HasPrefix(runtimeName, envIdentifier+"-") {
		return strings.TrimPrefix(runtimeName, envIdentifier+"-")
	}
	return ""
}

func dnsLabelPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if valid {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func valueFromMaps(key string, maps ...map[string]string) string {
	for _, values := range maps {
		if strings.TrimSpace(values[key]) != "" {
			return strings.TrimSpace(values[key])
		}
	}
	return ""
}

// paapLabels 返回标准的 PAAP 标签
func paapLabels(md serviceMetadata) map[string]string {
	labels := map[string]string{
		"paap.io/app":           md.AppIdentifier,
		"paap.io/env":           md.EnvIdentifier,
		"paap.io/tool":          md.Tool,
		"paap.io/service":       md.ServiceType,
		"paap.io/service-type":  md.ServiceType,
		"paap.io/category":      md.Category,
		"paap.io/resource-role": md.ResourceRole,
		"paap.io/managed-by":    "paap-operator",
	}
	return labels
}

// paapAnnotations 返回标准的 PAAP 注解
func paapAnnotations(md serviceMetadata) map[string]string {
	return map[string]string{
		"paap.io/tool-namespace":    md.ToolNamespace,
		"paap.io/service-namespace": md.ToolNamespace,
		"paap.io/app":               md.AppIdentifier,
		"paap.io/env":               md.EnvIdentifier,
		"paap.io/tool":              md.Tool,
		"paap.io/service":           md.ServiceType,
		"paap.io/service-type":      md.ServiceType,
		"paap.io/category":          md.Category,
		"paap.io/resource-role":     md.ResourceRole,
	}
}

func serviceMetadataWithResourceRole(md serviceMetadata, resourceRole string) serviceMetadata {
	md.ResourceRole = resourceRole
	return md
}

func (r *ServiceInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	svc := &paapv1.ServiceInstance{}
	if err := r.Get(ctx, req.NamespacedName, svc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	md := metadataFromServiceInstance(svc)
	appIdentifier := md.AppIdentifier
	envIdentifier := md.EnvIdentifier
	serviceType := md.ServiceType
	toolNS := md.ToolNamespace

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

	if expectedName := expectedConcreteServiceInstanceName(svc, md); expectedName != "" && svc.Name != expectedName {
		err := r.updateServiceInstanceStatus(ctx, svc, func(status *paapv1.ServiceInstanceStatus) {
			status.Phase = "Error"
			status.ObservedGeneration = svc.Generation
			status.Conditions = []metav1.Condition{{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "InvalidServiceInstanceName",
				Message:            fmt.Sprintf("service instance name must be %s for concrete tool identity", expectedName),
				ObservedGeneration: svc.Generation,
				LastTransitionTime: metav1.Now(),
			}}
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("ServiceInstance name does not match concrete tool identity", "name", svc.Name, "expected", expectedName)
		return ctrl.Result{}, nil
	}

	// 获取关联的 Environment
	env := &paapv1.Environment{}
	envKey := types.NamespacedName{
		Name:      svc.Spec.EnvironmentRef.Name,
		Namespace: svc.Namespace,
	}
	if err := r.Get(ctx, envKey, env); err != nil {
		if apierrors.IsNotFound(err) {
			_ = r.updateServiceInstanceStatus(ctx, svc, func(status *paapv1.ServiceInstanceStatus) {
				status.Phase = "Error"
			})
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// 检查 Environment 是否就绪
	if env.Status.Phase != "Running" {
		logger.Info("Environment not ready, waiting", "envPhase", env.Status.Phase)
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	if svc.Spec.Helm == nil {
		err := r.updateServiceInstanceStatus(ctx, svc, func(status *paapv1.ServiceInstanceStatus) {
			status.Phase = "Error"
			status.ObservedGeneration = svc.Generation
			status.Conditions = []metav1.Condition{{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "MissingHelmSpec",
				Message:            "service instances must define spec.helm",
				ObservedGeneration: svc.Generation,
				LastTransitionTime: metav1.Now(),
			}}
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("ServiceInstance missing Helm spec; marking Error", "type", svc.Spec.Type)
		return ctrl.Result{}, nil
	}

	// Step 1: 创建工具独占 namespace
	if err := r.ensureToolNamespace(ctx, toolNS, md); err != nil {
		logger.Error(err, "failed to ensure tool namespace", "namespace", toolNS)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// Step 2: 在工具 ns 内创建 SA
	if err := r.ensureServiceAccount(ctx, svc, toolNS, md); err != nil {
		logger.Error(err, "failed to ensure SA")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// Step 3: 在工具 ns 内创建 toolNamespaceRole（工具在自己 namespace 的权限）
	if svc.Spec.ToolNamespaceRole != nil {
		if err := r.ensureRole(ctx, svc, toolNS, "tool-ns", svc.Spec.ToolNamespaceRole, md); err != nil {
			logger.Error(err, "failed to ensure toolNamespaceRole")
		}
		r.ensureRoleBinding(ctx, svc, toolNS, "tool-ns", toolNS, md)
	}

	if svc.Spec.ClusterRole != nil && len(svc.Spec.ClusterRole.Rules) > 0 {
		if err := r.ensureClusterRole(ctx, svc, md); err != nil {
			logger.Error(err, "failed to ensure cluster Role")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		if err := r.ensureClusterRoleBinding(ctx, svc, md); err != nil {
			logger.Error(err, "failed to ensure cluster RoleBinding")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
	} else if err := r.deleteClusterRBAC(ctx, md); err != nil {
		logger.Error(err, "failed to delete stale cluster RBAC")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// Step 4: 发现环境内 namespace。Helm values 使用全环境 namespace 集合供日志/监控采集；
	// workload RBAC 只投射到业务 namespace，environment RBAC 投射到同环境其它 namespace。
	environmentNSList := r.discoverEnvironmentNamespaces(ctx, appIdentifier, envIdentifier)
	workloadNSList := r.discoverWorkloadNamespaces(ctx, appIdentifier, envIdentifier)
	workloadRBACNamespaces := workloadRBACTargetNamespaces(serviceType, svc.Spec.WorkloadRole, environmentNSList, workloadNSList)
	environmentRBACNamespaces := []string{}
	if svc.Spec.EnvironmentRole != nil && len(svc.Spec.EnvironmentRole.Rules) > 0 {
		environmentRBACNamespaces = environmentRBACTargetNamespaces(toolNS, environmentNSList)
	}
	rbacStatuses := make([]paapv1.RBACNamespaceStatus, 0, len(workloadRBACNamespaces)+len(environmentRBACNamespaces)+1)
	recordRBACStatus := func(ns string, roleCreated, bindingCreated bool) {
		for i := range rbacStatuses {
			if rbacStatuses[i].Namespace != ns {
				continue
			}
			rbacStatuses[i].RoleCreated = rbacStatuses[i].RoleCreated && roleCreated
			rbacStatuses[i].RoleBindingCreated = rbacStatuses[i].RoleBindingCreated && bindingCreated
			return
		}
		rbacStatuses = append(rbacStatuses, paapv1.RBACNamespaceStatus{
			Namespace:          ns,
			RoleCreated:        roleCreated,
			RoleBindingCreated: bindingCreated,
		})
	}
	recordRBACStatus(toolNS, true, true)

	if len(svc.Spec.WorkloadRole.Rules) > 0 {
		for _, nsName := range workloadRBACNamespaces {
			if nsName == toolNS {
				continue // 跳过工具自己的 ns
			}
			if err := r.ensureRole(ctx, svc, nsName, "workload", &svc.Spec.WorkloadRole, md); err != nil {
				logger.Error(err, "failed to ensure workload Role", "namespace", nsName)
				recordRBACStatus(nsName, false, false)
				continue
			}
			if err := r.ensureRoleBinding(ctx, svc, nsName, "workload", toolNS, md); err != nil {
				logger.Error(err, "failed to ensure workload RoleBinding", "namespace", nsName)
				recordRBACStatus(nsName, true, false)
				continue
			}
			recordRBACStatus(nsName, true, true)
		}
	}
	if svc.Spec.EnvironmentRole != nil && len(svc.Spec.EnvironmentRole.Rules) > 0 {
		for _, nsName := range environmentRBACNamespaces {
			if err := r.ensureRole(ctx, svc, nsName, "environment", svc.Spec.EnvironmentRole, md); err != nil {
				logger.Error(err, "failed to ensure environment Role", "namespace", nsName)
				recordRBACStatus(nsName, false, false)
				continue
			}
			if err := r.ensureRoleBinding(ctx, svc, nsName, "environment", toolNS, md); err != nil {
				logger.Error(err, "failed to ensure environment RoleBinding", "namespace", nsName)
				recordRBACStatus(nsName, true, false)
				continue
			}
			recordRBACStatus(nsName, true, true)
		}
	}

	// Step 5.1: 清理已移除 namespace 中的残留 RBAC
	workloadRoleName := fmt.Sprintf("%s-%s-%s-workload-manager", appIdentifier, envIdentifier, md.Tool)
	environmentRoleName := fmt.Sprintf("%s-%s-%s-environment-manager", appIdentifier, envIdentifier, md.Tool)
	if err := r.cleanupRemovedRoleRBAC(ctx, workloadRoleName, workloadRBACNamespaces, environmentNSList, svc.Status.RBACNamespaces); err != nil {
		logger.Error(err, "failed to clean removed workload RBAC")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}
	if err := r.cleanupRemovedRoleRBAC(ctx, environmentRoleName, environmentRBACNamespaces, environmentNSList, svc.Status.RBACNamespaces); err != nil {
		logger.Error(err, "failed to clean removed environment RBAC")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}
	if err := r.cleanupStaleRoleRBACForTool(ctx, workloadRoleName, environmentNSList, md, "workload"); err != nil {
		logger.Error(err, "failed to clean stale workload RBAC")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}
	if err := r.cleanupStaleRoleRBACForTool(ctx, environmentRoleName, environmentNSList, md, "environment"); err != nil {
		logger.Error(err, "failed to clean stale environment RBAC")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	if svc.Spec.Helm != nil && len(environmentNSList) > 0 && r.syncNamespaceValuesInHelmValues(ctx, svc, environmentNSList, workloadNSList) {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Step 6: 部署工具组件。已观测到当前 generation 为 Running 时不重复执行 Helm，
	// 避免每次 reconcile 都创建新 release revision 或撞上 pending 操作。
	if svc.Spec.Helm != nil && shouldEnsureHelmRelease(svc) {
		if err := r.ensureHelmRelease(ctx, svc); err != nil {
			if isRecoverableHelmRaceError(err) {
				logger.Info("helm release is being reconciled by another operation; using live component status", "error", err)
			} else if isHelmImmutableStatefulSetError(err) {
				components, ready, collectErr := r.collectToolComponentStatus(ctx, svc)
				if collectErr == nil && ready && len(components) > 0 {
					logger.Info("helm upgrade hit immutable StatefulSet fields but live workloads are ready; keeping live component status", "error", err)
				} else {
					_ = r.updateServiceInstanceStatus(ctx, svc, func(status *paapv1.ServiceInstanceStatus) {
						status.Phase = "Error"
					})
					return ctrl.Result{RequeueAfter: 5 * time.Second}, err
				}
			} else {
				_ = r.updateServiceInstanceStatus(ctx, svc, func(status *paapv1.ServiceInstanceStatus) {
					status.Phase = "Error"
				})
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
		}
	}

	if err := r.ensureToolComponents(ctx, svc); err != nil {
		logger.Error(err, "failed to ensure tool components")
	}

	components, toolReady, err := r.collectToolComponentStatus(ctx, svc)
	if err != nil {
		logger.Error(err, "failed to collect tool component status")
	}
	phase := "Running"
	if !toolReady {
		phase = "Installing"
	}

	if err := r.updateServiceInstanceStatus(ctx, svc, func(status *paapv1.ServiceInstanceStatus) {
		status.Phase = phase
		if toolReady {
			clearServiceInstanceCondition(status, "Ready")
		}
		status.ServiceAccount = &paapv1.ServiceAccountStatus{
			Name:      svc.Spec.ServiceAccount.Name,
			Namespace: toolNS,
			Created:   true,
		}
		status.RBACNamespaces = rbacStatuses
		status.Components = components
		status.ObservedGeneration = svc.Generation
	}); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: serviceInstanceStatusRequeueAfter(toolReady)}, nil
}

func serviceInstanceStatusRequeueAfter(toolReady bool) time.Duration {
	if toolReady {
		return 60 * time.Second
	}
	return 5 * time.Second
}

func isRecoverableHelmRaceError(err error) bool {
	return paaphelm.IsReleaseAlreadyExists(err) || paaphelm.IsReleaseOperationInProgress(err)
}

func isHelmImmutableStatefulSetError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "statefulset") &&
		strings.Contains(message, "forbidden") &&
		strings.Contains(message, "updates to statefulset spec")
}

func (r *ServiceInstanceReconciler) ensureHelmRelease(ctx context.Context, svc *paapv1.ServiceInstance) error {
	logger := log.FromContext(ctx)
	helmSpec := svc.Spec.Helm
	if helmSpec == nil {
		return nil
	}

	chartPath := helmSpec.ChartName
	if helmSpec.ChartArchivePath != "" || helmSpec.S3Bucket != "" {
		tmpDir, err := os.MkdirTemp("", "paap-helm-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		archivePath := helmSpec.ChartArchivePath
		if helmSpec.S3Bucket != "" && helmSpec.S3Key != "" {
			s3Client, err := k8s.NewS3Client("minio.paap-system.svc.cluster.local:9000", "minioadmin", "minioadmin123", helmSpec.S3Bucket, false)
			if err != nil {
				return err
			}
			archivePath = filepath.Join(tmpDir, "chart.tar.gz")
			if err := s3Client.DownloadFile(ctx, helmSpec.S3Key, archivePath); err != nil {
				return err
			}
		}
		if archivePath == "" {
			return fmt.Errorf("missing chart archive path")
		}
		if err := extractTarGz(archivePath, tmpDir); err != nil {
			return err
		}
		chartPath = filepath.Join(tmpDir, "chart")
	}

	presetValues := helmSpec.PresetValues
	if presetValues == "" && (helmSpec.ChartArchivePath != "" || helmSpec.S3Bucket != "") {
		presetPath := filepath.Join(filepath.Dir(chartPath), "preset-values.yaml")
		if data, err := os.ReadFile(filepath.Clean(presetPath)); err == nil {
			presetValues = string(data)
		}
	}
	values, err := paaphelm.BuildValues(presetValues, helmSpec.Values)
	if err != nil {
		return err
	}
	if err := r.applyDynamicCRDInstall(ctx, helmSpec, values); err != nil {
		return err
	}
	applyRuntimeRegistryValues(svc, values)

	if r.HelmClient == nil {
		r.HelmClient = paaphelm.NewClient()
	}
	if helmSpec.ChartRepo != "" {
		return fmt.Errorf("remote helm repositories are not supported by the operator yet; use S3 chart archives")
	}
	metadata := helmResourceMetadata(svc)
	err = r.HelmClient.UpgradeInstallWithMetadata(helmSpec.ReleaseName, helmSpec.Namespace, chartPath, values, metadata)
	if err != nil {
		logger.Error(err, "failed to install helm release", "release", helmSpec.ReleaseName, "namespace", helmSpec.Namespace)
		return err
	}
	return nil
}

func helmResourceMetadata(svc *paapv1.ServiceInstance) paaphelm.ResourceMetadata {
	md := serviceMetadataWithResourceRole(metadataFromServiceInstance(svc), "tool")
	toolNS := md.ToolNamespace
	labels := paapLabels(md)
	labels["paap.io/scope"] = "tool"
	labels["paap.io/tool-namespace"] = toolNS
	labels["paap.io/service-namespace"] = toolNS

	annotations := paapAnnotations(md)
	annotations["paap.io/scope"] = "tool"

	return paaphelm.ResourceMetadata{
		Namespace:   toolNS,
		Labels:      labels,
		Annotations: annotations,
	}
}

func shouldEnsureHelmRelease(svc *paapv1.ServiceInstance) bool {
	return svc.Status.ObservedGeneration != svc.Generation || len(svc.Status.Components) == 0
}

func (r *ServiceInstanceReconciler) syncEnvironmentNamespacesInHelmValues(ctx context.Context, svc *paapv1.ServiceInstance, environmentNamespaces []string) bool {
	return r.syncNamespaceValuesInHelmValues(ctx, svc, environmentNamespaces, environmentNamespaces)
}

func (r *ServiceInstanceReconciler) syncNamespaceValuesInHelmValues(ctx context.Context, svc *paapv1.ServiceInstance, environmentNamespaces, workloadNamespaces []string) bool {
	if svc.Spec.Helm == nil {
		return false
	}
	if svc.Spec.Helm.Values == nil {
		svc.Spec.Helm.Values = map[string]string{}
	}
	changed := false
	for key, expected := range namespaceHelmValueTargets(svc.Spec.Helm, environmentNamespaces, workloadNamespaces) {
		if svc.Spec.Helm.Values[key] != expected {
			svc.Spec.Helm.Values[key] = expected
			changed = true
		}
	}
	if !changed {
		return false
	}
	if err := r.Update(ctx, svc); err != nil {
		log.FromContext(ctx).Error(err, "failed to update environment namespace helm values")
		return false
	}
	return true
}

func stableNamespaceList(namespaces []string) []string {
	seen := make(map[string]bool, len(namespaces))
	result := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		ns = strings.TrimSpace(ns)
		if ns == "" || seen[ns] {
			continue
		}
		seen[ns] = true
		result = append(result, ns)
	}
	sort.Strings(result)
	return result
}

func environmentNamespaceHelmValueKeys(helmSpec *paapv1.HelmInstallSpec) []string {
	targets := namespaceHelmValueTargets(helmSpec, nil, nil)
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func namespaceHelmValueTargets(helmSpec *paapv1.HelmInstallSpec, environmentNamespaces, workloadNamespaces []string) map[string]string {
	envExpected := strings.Join(stableNamespaceList(environmentNamespaces), ",")
	workloadExpected := strings.Join(stableNamespaceList(workloadNamespaces), ",")
	targets := map[string]string{}
	if helmSpec == nil || helmSpec.PlatformManifest == "" {
		return targets
	}
	var manifest model.PlatformManifest
	if err := json.Unmarshal([]byte(helmSpec.PlatformManifest), &manifest); err != nil {
		return targets
	}
	for _, mapping := range manifest.VariableMapping {
		switch mapping.PlatformVar {
		case "env_namespaces", "all_namespaces":
			if mapping.HelmVar != "" {
				targets[mapping.HelmVar] = envExpected
			}
		case "workload_namespaces":
			if mapping.HelmVar != "" {
				targets[mapping.HelmVar] = workloadExpected
			}
		}
	}
	return targets
}

func (r *ServiceInstanceReconciler) collectToolComponentStatus(ctx context.Context, svc *paapv1.ServiceInstance) ([]paapv1.ToolComponentStatus, bool, error) {
	components := []paapv1.ToolComponentStatus{}
	ready := true
	var firstReason, firstMessage string

	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, client.InNamespace(svc.Spec.ToolNamespace)); err != nil {
		return nil, false, err
	}
	for _, deploy := range deployments.Items {
		desired := int32(0)
		if deploy.Spec.Replicas != nil {
			desired = *deploy.Spec.Replicas
		}
		componentReady := desired == deploy.Status.ReadyReplicas
		reason, message := componentStatusDetails(ctx, r.Client, svc.Spec.ToolNamespace, deploy.Name, componentReady, deploy.Status.ReadyReplicas, desired)
		if !componentReady {
			ready = false
			if firstReason == "" {
				firstReason, firstMessage = reason, message
			}
		}
		components = append(components, paapv1.ToolComponentStatus{
			Name:     deploy.Name,
			Kind:     "Deployment",
			Ready:    componentReady,
			Replicas: fmt.Sprintf("%d/%d", deploy.Status.ReadyReplicas, desired),
			Reason:   reason,
			Message:  message,
		})
	}

	statefulSets := &appsv1.StatefulSetList{}
	if err := r.List(ctx, statefulSets, client.InNamespace(svc.Spec.ToolNamespace)); err != nil {
		return nil, false, err
	}
	for _, sts := range statefulSets.Items {
		desired := int32(0)
		if sts.Spec.Replicas != nil {
			desired = *sts.Spec.Replicas
		}
		componentReady := desired == sts.Status.ReadyReplicas
		reason, message := componentStatusDetails(ctx, r.Client, svc.Spec.ToolNamespace, sts.Name, componentReady, sts.Status.ReadyReplicas, desired)
		if !componentReady {
			ready = false
			if firstReason == "" {
				firstReason, firstMessage = reason, message
			}
		}
		components = append(components, paapv1.ToolComponentStatus{
			Name:     sts.Name,
			Kind:     "StatefulSet",
			Ready:    componentReady,
			Replicas: fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, desired),
			Reason:   reason,
			Message:  message,
		})
	}

	daemonSets := &appsv1.DaemonSetList{}
	if err := r.List(ctx, daemonSets, client.InNamespace(svc.Spec.ToolNamespace)); err != nil {
		return nil, false, err
	}
	for _, ds := range daemonSets.Items {
		desired := ds.Status.DesiredNumberScheduled
		componentReady := desired == ds.Status.NumberReady
		reason, message := componentStatusDetails(ctx, r.Client, svc.Spec.ToolNamespace, ds.Name, componentReady, ds.Status.NumberReady, desired)
		if !componentReady {
			ready = false
			if firstReason == "" {
				firstReason, firstMessage = reason, message
			}
		}
		components = append(components, paapv1.ToolComponentStatus{
			Name:     ds.Name,
			Kind:     "DaemonSet",
			Ready:    componentReady,
			Replicas: fmt.Sprintf("%d/%d", ds.Status.NumberReady, desired),
			Reason:   reason,
			Message:  message,
		})
	}

	replicaSets := &appsv1.ReplicaSetList{}
	if err := r.List(ctx, replicaSets, client.InNamespace(svc.Spec.ToolNamespace)); err != nil {
		return nil, false, err
	}
	for _, rs := range replicaSets.Items {
		if !isActiveReplicaSet(rs) || replicaSetOwnedByDeployment(rs) {
			continue
		}
		desired := int32(0)
		if rs.Spec.Replicas != nil {
			desired = *rs.Spec.Replicas
		}
		componentReady := desired == rs.Status.ReadyReplicas
		reason, message := componentStatusDetails(ctx, r.Client, svc.Spec.ToolNamespace, rs.Name, componentReady, rs.Status.ReadyReplicas, desired)
		if !componentReady {
			ready = false
			if firstReason == "" {
				firstReason, firstMessage = reason, message
			}
		}
		components = append(components, paapv1.ToolComponentStatus{
			Name:     rs.Name,
			Kind:     "ReplicaSet",
			Ready:    componentReady,
			Replicas: fmt.Sprintf("%d/%d", rs.Status.ReadyReplicas, desired),
			Reason:   reason,
			Message:  message,
		})
	}

	if !ready {
		if firstReason == "" {
			firstReason = "ComponentsNotReady"
			firstMessage = fmt.Sprintf("tool namespace %s still has workloads not ready", svc.Spec.ToolNamespace)
		}
		setServiceInstanceCondition(&svc.Status, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             firstReason,
			Message:            firstMessage,
			ObservedGeneration: svc.Generation,
			LastTransitionTime: metav1.Now(),
		})
	}
	if len(components) == 0 {
		ready = false
		setServiceInstanceCondition(&svc.Status, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "NoToolWorkloads",
			Message:            fmt.Sprintf("tool namespace %s has no workloads yet", svc.Spec.ToolNamespace),
			ObservedGeneration: svc.Generation,
			LastTransitionTime: metav1.Now(),
		})
	}
	return components, ready, nil
}

func isActiveReplicaSet(rs appsv1.ReplicaSet) bool {
	if rs.Spec.Replicas != nil && *rs.Spec.Replicas > 0 {
		return true
	}
	if rs.Status.Replicas > 0 || rs.Status.ReadyReplicas > 0 || rs.Status.AvailableReplicas > 0 {
		return true
	}
	return false
}

func replicaSetOwnedByDeployment(rs appsv1.ReplicaSet) bool {
	for _, owner := range rs.OwnerReferences {
		if owner.Kind == "Deployment" && owner.APIVersion == "apps/v1" {
			return true
		}
	}
	return false
}

func componentStatusDetails(ctx context.Context, cl client.Client, namespace, workloadName string, ready bool, readyReplicas, desired int32) (string, string) {
	if ready {
		return "", ""
	}

	pods := &corev1.PodList{}
	if err := cl.List(ctx, pods, client.InNamespace(namespace)); err == nil {
		for _, pod := range pods.Items {
			if !strings.Contains(pod.Name, workloadName) {
				continue
			}
			for _, status := range pod.Status.ContainerStatuses {
				if status.State.Waiting != nil && status.State.Waiting.Reason != "" {
					return status.State.Waiting.Reason, firstNonEmptyString(status.State.Waiting.Message, fmt.Sprintf("container %s is waiting", status.Name))
				}
				if !status.Ready {
					return "ContainerNotReady", fmt.Sprintf("container %s in pod %s is not ready", status.Name, pod.Name)
				}
			}
		}
	}

	return "WorkloadNotReady", fmt.Sprintf("%s is not ready (%d/%d ready)", workloadName, readyReplicas, desired)
}

func setServiceInstanceCondition(status *paapv1.ServiceInstanceStatus, condition metav1.Condition) {
	for i := range status.Conditions {
		if status.Conditions[i].Type == condition.Type {
			status.Conditions[i] = condition
			return
		}
	}
	status.Conditions = append(status.Conditions, condition)
}

func clearServiceInstanceCondition(status *paapv1.ServiceInstanceStatus, conditionType string) {
	conditions := status.Conditions[:0]
	for _, condition := range status.Conditions {
		if condition.Type != conditionType {
			conditions = append(conditions, condition)
		}
	}
	status.Conditions = conditions
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (r *ServiceInstanceReconciler) updateServiceInstanceStatus(ctx context.Context, svc *paapv1.ServiceInstance, mutate func(*paapv1.ServiceInstanceStatus)) error {
	key := client.ObjectKeyFromObject(svc)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &paapv1.ServiceInstance{}
		if err := r.Get(ctx, key, latest); err != nil {
			return err
		}
		mutate(&latest.Status)
		return r.Status().Update(ctx, latest)
	})
}

func (r *ServiceInstanceReconciler) applyDynamicCRDInstall(ctx context.Context, helmSpec *paapv1.HelmInstallSpec, values map[string]interface{}) error {
	if !isArgoCDHelmInstall(helmSpec) {
		return nil
	}

	crds := []string{
		"applications.argoproj.io",
		"appprojects.argoproj.io",
		"applicationsets.argoproj.io",
	}
	install := false
	for _, name := range crds {
		var crd apiextensionsv1.CustomResourceDefinition
		err := r.Client.Get(ctx, types.NamespacedName{Name: name}, &crd)
		if apierrors.IsNotFound(err) {
			install = true
			break
		}
		if err != nil {
			return err
		}
	}
	setHelmNestedValue(values, "crds.install", install)
	return nil
}

func isArgoCDHelmInstall(helmSpec *paapv1.HelmInstallSpec) bool {
	if strings.Contains(strings.ToLower(helmSpec.S3Key), "argocd") ||
		strings.Contains(strings.ToLower(helmSpec.ChartName), "argocd") {
		return true
	}
	if helmSpec.PlatformManifest == "" {
		return false
	}
	var manifest model.PlatformManifest
	if err := json.Unmarshal([]byte(helmSpec.PlatformManifest), &manifest); err != nil {
		return false
	}
	return strings.EqualFold(manifest.Name, "argocd")
}

func setHelmNestedValue(values map[string]interface{}, key string, value interface{}) {
	parts := strings.Split(key, ".")
	current := values
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func applyRuntimeRegistryValues(svc *paapv1.ServiceInstance, values map[string]interface{}) {
	if svc == nil || values == nil {
		return
	}
	serviceType := strings.TrimSpace(svc.Labels["paap.io/service"])
	if serviceType == "" {
		serviceType = strings.TrimSpace(svc.Spec.Type)
	}
	if serviceType != "registry" && serviceType != "harbor" {
		return
	}
	appID := strings.TrimSpace(svc.Labels["paap.io/app"])
	envID := strings.TrimSpace(svc.Labels["paap.io/env"])
	if appID == "" || envID == "" {
		return
	}
	host := svcservice.RuntimeRegistryHost(model.Application{Identifier: appID}, model.Environment{Identifier: envID}, serviceType)
	if host == "" {
		return
	}
	tlsHost := registryTLSHost(host)
	switch serviceType {
	case "registry":
		// HTTP-only mode — chart defaults (tls.enabled: false, port 5000) are correct.
		// No runtime overrides needed.
	case "harbor":
		setHelmNestedValue(values, "externalURL", "https://"+host)
		setHelmNestedValue(values, "expose.ingress.hosts.core", tlsHost)
		setHelmNestedValue(values, "expose.tls.auto.commonName", tlsHost)
	}
}

func registryTLSHost(host string) string {
	host = strings.TrimSpace(host)
	if idx := strings.LastIndex(host, ":"); idx > -1 && !strings.Contains(host[idx+1:], "]") {
		return host[:idx]
	}
	return host
}

func extractTarGz(archivePath, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		name := strings.TrimPrefix(header.Name, "./")
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		targetPath := filepath.Join(targetDir, name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			out, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensureToolNamespace 创建工具独占的 namespace，打全量标签和注解
func (r *ServiceInstanceReconciler) ensureToolNamespace(ctx context.Context, nsName string, md serviceMetadata) error {
	md = serviceMetadataWithResourceRole(md, "tool")
	ns := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: nsName}, ns)
	if err == nil {
		md.ToolNamespace = nsName
		changed := mergeObjectLabels(ns, paapLabels(md))
		changed = mergeObjectLabels(ns, map[string]string{"paap-managed": "true"}) || changed
		changed = mergeObjectAnnotations(ns, paapAnnotations(md)) || changed
		if ns.Labels["paap.io/role"] != "tool" {
			if ns.Labels == nil {
				ns.Labels = make(map[string]string)
			}
			ns.Labels["paap.io/role"] = "tool"
			changed = true
		}
		if changed {
			return r.Update(ctx, ns)
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	md.ToolNamespace = nsName
	labels := paapLabels(md)
	labels["paap.io/role"] = "tool" // 标记为工具 namespace
	labels["paap-managed"] = "true"

	annotations := paapAnnotations(md)

	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nsName,
			Labels:      labels,
			Annotations: annotations,
		},
	}
	return r.Create(ctx, ns)
}

type labeledAnnotatedObject interface {
	GetLabels() map[string]string
	SetLabels(map[string]string)
	GetAnnotations() map[string]string
	SetAnnotations(map[string]string)
}

func mergeObjectLabels(obj labeledAnnotatedObject, labels map[string]string) bool {
	current := obj.GetLabels()
	if current == nil {
		current = make(map[string]string)
	}
	changed := false
	for k, v := range labels {
		if current[k] != v {
			current[k] = v
			changed = true
		}
	}
	obj.SetLabels(current)
	return changed
}

func mergeObjectAnnotations(obj labeledAnnotatedObject, annotations map[string]string) bool {
	current := obj.GetAnnotations()
	if current == nil {
		current = make(map[string]string)
	}
	changed := false
	for k, v := range annotations {
		if current[k] != v {
			current[k] = v
			changed = true
		}
	}
	obj.SetAnnotations(current)
	return changed
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
		if isReservedKubernetesNamespace(ns.Name) {
			continue
		}
		result = append(result, ns.Name)
	}
	sort.Strings(result)
	return result
}

// discoverEnvironmentNamespaces returns every namespace that belongs to the
// current application/environment, including tool namespaces.
func (r *ServiceInstanceReconciler) discoverEnvironmentNamespaces(ctx context.Context, appIdentifier, envIdentifier string) []string {
	nsList := &corev1.NamespaceList{}
	labels := client.MatchingLabels{
		"paap.io/app": appIdentifier,
		"paap.io/env": envIdentifier,
	}
	if err := r.List(ctx, nsList, labels); err != nil {
		return nil
	}
	result := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		if isReservedKubernetesNamespace(ns.Name) {
			continue
		}
		result = append(result, ns.Name)
	}
	sort.Strings(result)
	return result
}

func isReservedKubernetesNamespace(namespace string) bool {
	switch strings.TrimSpace(namespace) {
	case "default", "kube-system", "kube-public", "kube-node-lease":
		return true
	default:
		return false
	}
}

func workloadRBACTargetNamespaces(toolType string, role paapv1.RoleSpec, environmentNamespaces, workloadNamespaces []string) []string {
	if len(role.Rules) == 0 {
		return nil
	}
	return stableNamespaceList(workloadNamespaces)
}

func environmentRBACTargetNamespaces(toolNamespace string, environmentNamespaces []string) []string {
	result := make([]string, 0, len(environmentNamespaces))
	for _, ns := range stableNamespaceList(environmentNamespaces) {
		if ns == toolNamespace {
			continue
		}
		result = append(result, ns)
	}
	return result
}

func (r *ServiceInstanceReconciler) cleanupRemovedRoleRBAC(ctx context.Context, roleName string, currentNamespaces, environmentNamespaces []string, previous []paapv1.RBACNamespaceStatus) error {
	currentNSSet := make(map[string]bool)
	for _, ns := range currentNamespaces {
		currentNSSet[ns] = true
	}
	namespaces := make(map[string]bool)
	for _, ns := range environmentNamespaces {
		namespaces[ns] = true
	}
	for _, prev := range previous {
		namespaces[prev.Namespace] = true
	}
	for nsName := range namespaces {
		if currentNSSet[nsName] {
			continue
		}
		role := &rbacv1.Role{}
		if err := r.Get(ctx, types.NamespacedName{Name: roleName, Namespace: nsName}, role); err == nil {
			if err := r.Delete(ctx, role); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		} else if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		rb := &rbacv1.RoleBinding{}
		if err := r.Get(ctx, types.NamespacedName{Name: roleName, Namespace: nsName}, rb); err == nil {
			if err := r.Delete(ctx, rb); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		} else if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *ServiceInstanceReconciler) cleanupStaleRoleRBACForTool(ctx context.Context, currentName string, namespaces []string, md serviceMetadata, roleType string) error {
	for _, nsName := range stableNamespaceList(namespaces) {
		roles := &rbacv1.RoleList{}
		if err := r.List(ctx, roles, client.InNamespace(nsName)); err != nil {
			return err
		}
		for i := range roles.Items {
			role := &roles.Items[i]
			if !isStaleRoleRBACObject(role, currentName, md, roleType) {
				continue
			}
			if err := r.Delete(ctx, role); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}

		bindings := &rbacv1.RoleBindingList{}
		if err := r.List(ctx, bindings, client.InNamespace(nsName)); err != nil {
			return err
		}
		for i := range bindings.Items {
			binding := &bindings.Items[i]
			if !isStaleRoleRBACObject(binding, currentName, md, roleType) {
				continue
			}
			if err := r.Delete(ctx, binding); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func isStaleRoleRBACObject(obj client.Object, currentName string, md serviceMetadata, roleType string) bool {
	name := obj.GetName()
	if name == currentName || !strings.HasSuffix(name, "-"+roleType+"-manager") {
		return false
	}
	labels := obj.GetLabels()
	annotations := obj.GetAnnotations()
	managedBy := valueFromMaps("paap.io/managed-by", labels, annotations)
	if managedBy != "" && managedBy != "paap-operator" {
		return false
	}
	if valueFromMaps("paap.io/app", labels, annotations) != md.AppIdentifier ||
		valueFromMaps("paap.io/env", labels, annotations) != md.EnvIdentifier {
		return false
	}
	objToolNS := valueFromMaps("paap.io/tool-namespace", annotations, labels)
	if objToolNS == "" {
		objToolNS = valueFromMaps("paap.io/service-namespace", annotations, labels)
	}
	if objToolNS != md.ToolNamespace {
		return false
	}
	objServiceType := valueFromMaps("paap.io/service-type", labels, annotations)
	if objServiceType == "" {
		objServiceType = valueFromMaps("paap.io/service", labels, annotations)
	}
	return objServiceType == "" || objServiceType == md.ServiceType
}

func (r *ServiceInstanceReconciler) handleDeletion(ctx context.Context, svc *paapv1.ServiceInstance) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(svc, svcFinalizer) {
		return ctrl.Result{}, nil
	}

	md := metadataFromServiceInstance(svc)
	toolNS := md.ToolNamespace
	appIdentifier := md.AppIdentifier
	envIdentifier := md.EnvIdentifier

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

	// 清理环境 namespace 中投射的 workload/environment Role 和 RoleBinding。
	environmentNSList := r.discoverEnvironmentNamespaces(ctx, appIdentifier, envIdentifier)
	roleNames := []string{
		fmt.Sprintf("%s-%s-%s-workload-manager", appIdentifier, envIdentifier, md.Tool),
		fmt.Sprintf("%s-%s-%s-environment-manager", appIdentifier, envIdentifier, md.Tool),
	}
	for _, roleName := range roleNames {
		for _, nsName := range environmentNSList {
			if nsName == toolNS {
				continue
			}
			role := &rbacv1.Role{}
			if err := r.Get(ctx, types.NamespacedName{Name: roleName, Namespace: nsName}, role); err == nil {
				r.Delete(ctx, role)
			}
			rb := &rbacv1.RoleBinding{}
			if err := r.Get(ctx, types.NamespacedName{Name: roleName, Namespace: nsName}, rb); err == nil {
				r.Delete(ctx, rb)
			}
		}
	}

	if err := r.deleteClusterRBAC(ctx, md); err != nil {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, err
	}

	// 移除 Finalizer
	controllerutil.RemoveFinalizer(svc, svcFinalizer)
	if err := r.Update(ctx, svc); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("ServiceInstance deleted", "name", svc.Name, "type", md.ServiceType, "tool", md.Tool)
	return ctrl.Result{}, nil
}

func (r *ServiceInstanceReconciler) ensureServiceAccount(ctx context.Context, svc *paapv1.ServiceInstance, nsName string, md serviceMetadata) error {
	md = serviceMetadataWithResourceRole(md, "tool")
	md.ToolNamespace = svc.Spec.ToolNamespace
	sa := &corev1.ServiceAccount{}
	saKey := types.NamespacedName{
		Name:      svc.Spec.ServiceAccount.Name,
		Namespace: nsName,
	}
	if err := r.Get(ctx, saKey, sa); err == nil {
		changed := mergeObjectLabels(sa, paapLabels(md))
		changed = mergeObjectAnnotations(sa, paapAnnotations(md)) || changed
		if changed {
			return r.Update(ctx, sa)
		}
		return nil
	}

	labels := paapLabels(md)
	annotations := paapAnnotations(md)

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

func (r *ServiceInstanceReconciler) ensureRole(ctx context.Context, svc *paapv1.ServiceInstance, nsName, roleType string, roleSpec *paapv1.RoleSpec, md serviceMetadata) error {
	md = serviceMetadataWithResourceRole(md, "tool")
	md.ToolNamespace = svc.Spec.ToolNamespace
	roleName := fmt.Sprintf("%s-%s-%s-%s-manager", md.AppIdentifier, md.EnvIdentifier, md.Tool, roleType)
	role := &rbacv1.Role{}
	roleKey := types.NamespacedName{Name: roleName, Namespace: nsName}

	rules := make([]rbacv1.PolicyRule, 0, len(roleSpec.Rules))
	for _, rule := range roleSpec.Rules {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: rule.APIGroups,
			Resources: rule.Resources,
			Verbs:     rule.Verbs,
		})
	}

	if err := r.Get(ctx, roleKey, role); err == nil {
		// Role 已存在，检查规则是否需要更新
		changed := mergeObjectLabels(role, paapLabels(md))
		changed = mergeObjectAnnotations(role, paapAnnotations(md)) || changed
		if rulesEqual(role.Rules, rules) && !changed {
			return nil // 规则一致，无需更新
		}
		// 规则变更，更新 Role
		role.Rules = rules
		return r.Update(ctx, role)
	}

	logger := log.FromContext(ctx)
	logger.Info("creating Role", "name", roleName, "namespace", nsName, "type", roleType)

	labels := paapLabels(md)
	annotations := paapAnnotations(md)

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

// rulesEqual compares two slices of PolicyRule for equality.
func rulesEqual(a, b []rbacv1.PolicyRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !stringSliceEqual(a[i].APIGroups, b[i].APIGroups) ||
			!stringSliceEqual(a[i].Resources, b[i].Resources) ||
			!stringSliceEqual(a[i].Verbs, b[i].Verbs) {
			return false
		}
	}
	return true
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *ServiceInstanceReconciler) ensureRoleBinding(ctx context.Context, svc *paapv1.ServiceInstance, nsName, roleType, saNamespace string, md serviceMetadata) error {
	md = serviceMetadataWithResourceRole(md, "tool")
	md.ToolNamespace = svc.Spec.ToolNamespace
	roleName := fmt.Sprintf("%s-%s-%s-%s-manager", md.AppIdentifier, md.EnvIdentifier, md.Tool, roleType)
	rbName := roleName
	roleRef := rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "Role",
		Name:     roleName,
	}
	subjects := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      svc.Spec.ServiceAccount.Name,
			Namespace: saNamespace,
		},
	}
	rb := &rbacv1.RoleBinding{}
	rbKey := types.NamespacedName{Name: rbName, Namespace: nsName}
	if err := r.Get(ctx, rbKey, rb); err == nil {
		changed := mergeObjectLabels(rb, paapLabels(md))
		changed = mergeObjectAnnotations(rb, paapAnnotations(md)) || changed
		if rb.RoleRef != roleRef {
			rb.RoleRef = roleRef
			changed = true
		}
		if !subjectsEqual(rb.Subjects, subjects) {
			rb.Subjects = subjects
			changed = true
		}
		if changed {
			return r.Update(ctx, rb)
		}
		return nil
	}

	labels := paapLabels(md)
	annotations := paapAnnotations(md)

	rb = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rbName,
			Namespace:   nsName,
			Labels:      labels,
			Annotations: annotations,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}
	return r.Create(ctx, rb)
}

func subjectsEqual(a, b []rbacv1.Subject) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *ServiceInstanceReconciler) ensureClusterRole(ctx context.Context, svc *paapv1.ServiceInstance, md serviceMetadata) error {
	md = serviceMetadataWithResourceRole(md, "tool")
	md.ToolNamespace = svc.Spec.ToolNamespace
	clusterRoleName := fmt.Sprintf("%s-%s-%s-cluster-manager", md.AppIdentifier, md.EnvIdentifier, md.Tool)
	clusterRole := &rbacv1.ClusterRole{}
	rules := make([]rbacv1.PolicyRule, 0, len(svc.Spec.ClusterRole.Rules))
	for _, rule := range svc.Spec.ClusterRole.Rules {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: rule.APIGroups,
			Resources: rule.Resources,
			Verbs:     rule.Verbs,
		})
	}

	if err := r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, clusterRole); err == nil {
		changed := mergeObjectLabels(clusterRole, paapLabels(md))
		changed = mergeObjectAnnotations(clusterRole, paapAnnotations(md)) || changed
		if rulesEqual(clusterRole.Rules, rules) && !changed {
			return nil
		}
		clusterRole.Rules = rules
		return r.Update(ctx, clusterRole)
	}

	clusterRole = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        clusterRoleName,
			Labels:      paapLabels(md),
			Annotations: paapAnnotations(md),
		},
		Rules: rules,
	}
	return r.Create(ctx, clusterRole)
}

func (r *ServiceInstanceReconciler) ensureClusterRoleBinding(ctx context.Context, svc *paapv1.ServiceInstance, md serviceMetadata) error {
	md = serviceMetadataWithResourceRole(md, "tool")
	md.ToolNamespace = svc.Spec.ToolNamespace
	clusterRoleName := fmt.Sprintf("%s-%s-%s-cluster-manager", md.AppIdentifier, md.EnvIdentifier, md.Tool)
	crb := &rbacv1.ClusterRoleBinding{}
	if err := r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, crb); err == nil {
		changed := mergeObjectLabels(crb, paapLabels(md))
		changed = mergeObjectAnnotations(crb, paapAnnotations(md)) || changed
		if changed {
			return r.Update(ctx, crb)
		}
		return nil
	}

	crb = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        clusterRoleName,
			Labels:      paapLabels(md),
			Annotations: paapAnnotations(md),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      svc.Spec.ServiceAccount.Name,
				Namespace: svc.Spec.ServiceAccount.Namespace,
			},
		},
	}
	return r.Create(ctx, crb)
}

func (r *ServiceInstanceReconciler) deleteClusterRBAC(ctx context.Context, md serviceMetadata) error {
	clusterRoleName := fmt.Sprintf("%s-%s-%s-cluster-manager", md.AppIdentifier, md.EnvIdentifier, md.Tool)
	crb := &rbacv1.ClusterRoleBinding{}
	if err := r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, crb); err == nil {
		if err := r.Delete(ctx, crb); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	cr := &rbacv1.ClusterRole{}
	if err := r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, cr); err == nil {
		if err := r.Delete(ctx, cr); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	return nil
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

	md := serviceMetadataWithResourceRole(metadataFromServiceInstance(svc), "tool")
	toolNS := md.ToolNamespace

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
		for key, value := range paapLabels(md) {
			labels[key] = value
		}
		labels["paap.io/scope"] = "tool"
		labels["paap.io/tool-namespace"] = toolNS
		labels["paap.io/service-namespace"] = toolNS
		obj.SetLabels(labels)

		// 添加标准注解
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		for key, value := range paapAnnotations(md) {
			annotations[key] = value
		}
		annotations["paap.io/scope"] = "tool"
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
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(r.serviceInstancesForNamespace)).
		Complete(r)
}

func (r *ServiceInstanceReconciler) serviceInstancesForNamespace(ctx context.Context, obj client.Object) []reconcile.Request {
	if obj == nil {
		return nil
	}
	labels := obj.GetLabels()
	appIdentifier := labels["paap.io/app"]
	envIdentifier := labels["paap.io/env"]
	if appIdentifier == "" || envIdentifier == "" {
		return nil
	}

	svcList := &paapv1.ServiceInstanceList{}
	if err := r.List(ctx, svcList, client.MatchingLabels{
		"paap.io/app": appIdentifier,
		"paap.io/env": envIdentifier,
	}); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(svcList.Items))
	for _, svc := range svcList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      svc.Name,
				Namespace: svc.Namespace,
			},
		})
	}
	return requests
}
