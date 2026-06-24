package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	paapv1 "paap/api/v1"
	"paap/internal/model"
)

var (
	scheme    = runtime.NewScheme()
	k8sClient client.Client
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(paapv1.AddToScheme(scheme))
}

// Init initializes the K8s client
func Init() error {
	config, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get k8s config: %w", err)
	}

	k8sClient, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	return nil
}

// GetClient returns the K8s client
func GetClient() client.Client {
	return k8sClient
}

// SetClient overrides the package-level Kubernetes client.
// It is primarily used in tests and by callers that want to inject a fake client.
func SetClient(cl client.Client) {
	k8sClient = cl
}

func requireClient() (client.Client, error) {
	if k8sClient == nil {
		return nil, fmt.Errorf("k8s client not initialized")
	}
	return k8sClient, nil
}

// discoverService finds a service in namespace whose name contains keyword and
// returns its cluster DNS address (service.namespace.svc.cluster.local:port).
// If no matching service is found, it returns the fallback address.
func discoverService(ctx context.Context, namespace, keyword, fallbackAddr string) string {
	cl, err := requireClient()
	if err != nil {
		return fallbackAddr
	}
	var list corev1.ServiceList
	if err := cl.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return fallbackAddr
	}
	fport := fallbackPort(fallbackAddr)
	for _, svc := range list.Items {
		if serviceMatchesKeyword(svc, keyword) {
			port := pickPort(svc.Spec.Ports, fport)
			return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, namespace, port)
		}
	}
	return fallbackAddr
}

func serviceMatchesKeyword(svc corev1.Service, keyword string) bool {
	if strings.Contains(svc.Name, keyword) {
		return true
	}
	for _, value := range svc.Labels {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}

// discoverServiceExact tries to find a service whose name ends with suffix.
// If not found, falls back to discoverService with keyword = suffix.
func discoverServiceExact(ctx context.Context, namespace, suffix, fallbackAddr string) string {
	cl, err := requireClient()
	if err != nil {
		return fallbackAddr
	}
	var list corev1.ServiceList
	if err := cl.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return fallbackAddr
	}
	fport := fallbackPort(fallbackAddr)
	// First pass: exact suffix match (e.g., "-prometheus")
	for _, svc := range list.Items {
		if strings.HasSuffix(svc.Name, suffix) {
			port := pickPort(svc.Spec.Ports, fport)
			return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, namespace, port)
		}
	}
	// Second pass: contains keyword
	for _, svc := range list.Items {
		if strings.Contains(svc.Name, suffix) {
			port := pickPort(svc.Spec.Ports, fport)
			return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, namespace, port)
		}
	}
	return fallbackAddr
}

// pickPort chooses the port that matches preferredPort if available,
// otherwise the first defined port, or 80 as last resort.
func pickPort(ports []corev1.ServicePort, preferredPort int) int {
	if len(ports) == 0 {
		return 80
	}
	for _, p := range ports {
		if int(p.Port) == preferredPort {
			return preferredPort
		}
	}
	return int(ports[0].Port)
}

func fallbackPort(addr string) int {
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		return 80
	}
	p, _ := strconv.Atoi(addr[idx+1:])
	if p == 0 {
		return 80
	}
	return p
}

// discoverGrafanaCreds reads the Grafana admin credentials from the Helm-generated
// secret in the given namespace. Falls back to admin/admin if not found.
func discoverGrafanaCreds(ctx context.Context, namespace string) (user, pass string) {
	cl, err := requireClient()
	if err != nil {
		return "admin", "admin"
	}
	var list corev1.SecretList
	if err := cl.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return "admin", "admin"
	}
	for _, sec := range list.Items {
		if strings.Contains(sec.Name, "grafana") {
			u := string(sec.Data["admin-user"])
			p := string(sec.Data["admin-password"])
			if u != "" && p != "" {
				return u, p
			}
		}
	}
	return "admin", "admin"
}

// CreateConfigMap creates a ConfigMap in the specified namespace
func CreateConfigMap(ctx context.Context, namespace, name string, data map[string]string, labels map[string]string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
	return cl.Create(ctx, cm)
}

// CreateApplicationCR creates an Application CR in paap-system namespace
func CreateApplicationCR(ctx context.Context, name, identifier, description string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	app := &paapv1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      identifier,
			Namespace: "paap-system",
		},
		Spec: paapv1.ApplicationSpec{
			Name:        name,
			Identifier:  identifier,
			Description: description,
		},
	}
	return cl.Create(ctx, app)
}

// DeleteApplicationCR deletes an Application CR
func DeleteApplicationCR(ctx context.Context, identifier string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	app := &paapv1.Application{}
	key := types.NamespacedName{Name: identifier, Namespace: "paap-system"}
	if err := cl.Get(ctx, key, app); err != nil {
		return client.IgnoreNotFound(err)
	}
	return cl.Delete(ctx, app)
}

// CreateEnvironmentCR creates an Environment CR in the app's CR namespace
func CreateEnvironmentCR(ctx context.Context, appIdentifier, envName, envIdentifier, primaryNS string, additionalNS []paapv1.AdditionalNamespace, resourceQuota *paapv1.ResourceQuotaSpec) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	env := &paapv1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      envIdentifier,
			Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
			Labels: map[string]string{
				"paap.io/app": appIdentifier,
				"paap.io/env": envIdentifier,
			},
		},
		Spec: paapv1.EnvironmentSpec{
			Name:                 envName,
			Identifier:           envIdentifier,
			PrimaryNamespace:     primaryNS,
			AdditionalNamespaces: additionalNS,
			Network: paapv1.NetworkSpec{
				Isolation: "NetworkPolicy",
			},
			ResourceQuota: resourceQuota,
		},
	}
	return cl.Create(ctx, env)
}

// DeleteEnvironmentCR deletes an Environment CR
func DeleteEnvironmentCR(ctx context.Context, appIdentifier, envIdentifier string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	env := &paapv1.Environment{}
	key := types.NamespacedName{
		Name:      envIdentifier,
		Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
	}
	if err := cl.Get(ctx, key, env); err != nil {
		return client.IgnoreNotFound(err)
	}
	return cl.Delete(ctx, env)
}

// DeleteEnvironmentScopedResources removes CRs and namespaces owned by one
// application environment. It is intentionally label-driven so cleanup still
// works if database rows are already missing or stale.
func DeleteEnvironmentScopedResources(ctx context.Context, appIdentifier, envIdentifier string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	appIdentifier = strings.TrimSpace(appIdentifier)
	envIdentifier = strings.TrimSpace(envIdentifier)
	if appIdentifier == "" || envIdentifier == "" {
		return fmt.Errorf("appIdentifier and envIdentifier are required")
	}
	appNamespace := fmt.Sprintf("paap-app-%s", appIdentifier)
	selector := client.MatchingLabels{
		"paap.io/app": appIdentifier,
		"paap.io/env": envIdentifier,
	}

	svcList := &paapv1.ServiceInstanceList{}
	if err := cl.List(ctx, svcList, client.InNamespace(appNamespace), selector); err != nil {
		return err
	}
	for i := range svcList.Items {
		if err := cl.Delete(ctx, &svcList.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	compList := &paapv1.ComponentList{}
	if err := cl.List(ctx, compList, client.InNamespace(appNamespace), selector); err != nil {
		return err
	}
	for i := range compList.Items {
		if err := cl.Delete(ctx, &compList.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	if err := DeleteEnvironmentCR(ctx, appIdentifier, envIdentifier); err != nil {
		return err
	}

	nsList := &corev1.NamespaceList{}
	if err := cl.List(ctx, nsList, selector); err != nil {
		return err
	}
	for i := range nsList.Items {
		if err := cl.Delete(ctx, &nsList.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// DeleteApplicationScopedResources removes all namespaces and CR namespace
// objects owned by one application.
func DeleteApplicationScopedResources(ctx context.Context, appIdentifier string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	appIdentifier = strings.TrimSpace(appIdentifier)
	if appIdentifier == "" {
		return fmt.Errorf("appIdentifier is required")
	}
	selector := client.MatchingLabels{"paap.io/app": appIdentifier}
	nsList := &corev1.NamespaceList{}
	if err := cl.List(ctx, nsList, selector); err != nil {
		return err
	}
	for i := range nsList.Items {
		if err := cl.Delete(ctx, &nsList.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// CreateServiceInstanceCR creates a ServiceInstance CR
func CreateServiceInstanceCR(ctx context.Context, appIdentifier, envIdentifier, svcType string, workloadRole, toolNamespaceRole paapv1.RoleSpec, environmentRole, clusterRole *paapv1.RoleSpec, manifestsRef *paapv1.ConfigMapReference, helmSpec *paapv1.HelmInstallSpec, labels, annotations map[string]string) error {
	return UpsertServiceInstanceCR(ctx, appIdentifier, envIdentifier, svcType, workloadRole, toolNamespaceRole, environmentRole, clusterRole, manifestsRef, helmSpec, labels, annotations)
}

// UpsertServiceInstanceCR creates or refreshes a ServiceInstance CR. Refreshing
// is used after built-in template sync so existing tool instances pick up new
// Helm values, permissions, and chart metadata without a delete/reinstall cycle.
func UpsertServiceInstanceCR(ctx context.Context, appIdentifier, envIdentifier, svcType string, workloadRole, toolNamespaceRole paapv1.RoleSpec, environmentRole, clusterRole *paapv1.RoleSpec, manifestsRef *paapv1.ConfigMapReference, helmSpec *paapv1.HelmInstallSpec, labels, annotations map[string]string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return upsertServiceInstanceCROnce(ctx, appIdentifier, envIdentifier, svcType, workloadRole, toolNamespaceRole, environmentRole, clusterRole, manifestsRef, helmSpec, labels, annotations)
	})
}

func upsertServiceInstanceCROnce(ctx context.Context, appIdentifier, envIdentifier, svcType string, workloadRole, toolNamespaceRole paapv1.RoleSpec, environmentRole, clusterRole *paapv1.RoleSpec, manifestsRef *paapv1.ConfigMapReference, helmSpec *paapv1.HelmInstallSpec, labels, annotations map[string]string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	defaultToolNS := fmt.Sprintf("%s-%s-%s", appIdentifier, envIdentifier, strings.TrimSpace(svcType))
	toolNS := defaultToolNS
	if helmSpec != nil && strings.TrimSpace(helmSpec.Namespace) != "" {
		toolNS = strings.TrimSpace(helmSpec.Namespace)
	}
	namespace := fmt.Sprintf("paap-app-%s", appIdentifier)
	baseLabels := map[string]string{
		"paap.io/app":           appIdentifier,
		"paap.io/env":           envIdentifier,
		"paap.io/service":       svcType,
		"paap.io/service-type":  svcType,
		"paap.io/tool":          svcType,
		"paap.io/category":      "tool",
		"paap.io/resource-role": "service-instance",
		"paap.io/managed-by":    "paap-server",
	}
	mergeStringMap(baseLabels, labels)
	name := serviceInstanceCRName(envIdentifier, svcType, helmSpec, baseLabels)
	baseAnnotations := serviceInstanceAnnotations(toolNS, appIdentifier, envIdentifier, svcType, baseLabels)
	mergeStringMap(baseAnnotations, annotations)

	svc := &paapv1.ServiceInstance{}
	key := types.NamespacedName{Name: name, Namespace: namespace}
	exists := true
	if err := cl.Get(ctx, key, svc); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		exists = false
		svc = nil
	}
	if helmSpec != nil {
		helmSpec = helmSpec.DeepCopy()
		rewriteToolNamespaceValues(helmSpec, toolNS)
	}

	saName := serviceInstanceServiceAccountName(helmSpec, toolNS)
	spec := paapv1.ServiceInstanceSpec{
		EnvironmentRef:    paapv1.ObjectReference{Name: envIdentifier},
		Type:              svcType,
		ToolNamespace:     toolNS,
		ServiceAccount:    paapv1.ServiceAccountSpec{Name: saName, Namespace: toolNS},
		ToolNamespaceRole: &toolNamespaceRole,
		ClusterRole:       clusterRole,
		WorkloadRole:      workloadRole,
		EnvironmentRole:   environmentRole,
		ManifestsRef:      manifestsRef,
		Helm:              helmSpec,
	}

	if !exists {
		svc = &paapv1.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    baseLabels,
				Annotations: func() map[string]string {
					createdAnnotations := map[string]string{}
					for key, value := range baseAnnotations {
						createdAnnotations[key] = value
					}
					createdAnnotations["paap.io/template-synced-at"] = time.Now().UTC().Format(time.RFC3339Nano)
					return createdAnnotations
				}(),
			},
			Spec: spec,
		}
		return cl.Create(ctx, svc)
	}

	changed := false
	if svc.Labels == nil {
		svc.Labels = map[string]string{}
		changed = true
	}
	for key, value := range baseLabels {
		if svc.Labels[key] != value {
			svc.Labels[key] = value
			changed = true
		}
	}
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
		changed = true
	}
	for key, value := range baseAnnotations {
		if svc.Annotations[key] != value {
			svc.Annotations[key] = value
			changed = true
		}
	}
	if serviceInstanceSpecNeedsRefresh(svc.Spec, spec) {
		svc.Spec = spec
		svc.Annotations["paap.io/template-synced-at"] = time.Now().UTC().Format(time.RFC3339Nano)
		changed = true
	}
	if !changed {
		return nil
	}
	return cl.Update(ctx, svc)
}

func mergeStringMap(dst, src map[string]string) {
	for key, value := range src {
		if strings.TrimSpace(value) == "" {
			continue
		}
		dst[key] = value
	}
}

func serviceInstanceCRName(envIdentifier, svcType string, helmSpec *paapv1.HelmInstallSpec, labels map[string]string) string {
	identity := ""
	if labels != nil {
		identity = strings.TrimSpace(labels["paap.io/tool"])
	}
	derived := ""
	if helmSpec != nil {
		derived = serviceIdentityFromRuntimeName(envIdentifier, helmSpec.ReleaseName)
		if derived == "" {
			derived = serviceIdentityFromRuntimeName(envIdentifier, helmSpec.Namespace)
		}
	}
	if identity == "" || strings.EqualFold(identity, svcType) {
		if derived != "" {
			identity = derived
		}
	}
	if identity == "" {
		identity = svcType
	}
	return dnsLabelPart(envIdentifier + "-" + identity)
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

func serviceInstanceServiceAccountName(helmSpec *paapv1.HelmInstallSpec, toolNS string) string {
	if helmSpec != nil {
		for _, key := range toolNamespaceServiceAccountValueKeys(helmSpec) {
			if value := strings.TrimSpace(helmSpec.Values[key]); value != "" {
				return value
			}
		}
	}
	return fmt.Sprintf("%s-paap-manager", toolNS)
}

func toolNamespaceServiceAccountValueKeys(helmSpec *paapv1.HelmInstallSpec) []string {
	if helmSpec == nil || helmSpec.PlatformManifest == "" {
		return nil
	}
	var manifest model.PlatformManifest
	if err := json.Unmarshal([]byte(helmSpec.PlatformManifest), &manifest); err != nil {
		return nil
	}
	keys := make([]string, 0, len(manifest.VariableMapping))
	for _, mapping := range manifest.VariableMapping {
		if mapping.PlatformVar != "tool_namespace" || !isServiceAccountValueKey(mapping.HelmVar) {
			continue
		}
		keys = append(keys, mapping.HelmVar)
	}
	return keys
}

func isServiceAccountValueKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(key, "serviceaccount")
}

func rewriteToolNamespaceValues(helmSpec *paapv1.HelmInstallSpec, toolNS string) {
	if helmSpec == nil || helmSpec.Values == nil || strings.TrimSpace(toolNS) == "" {
		return
	}
	for _, key := range toolNamespaceValueKeys(helmSpec) {
		if _, ok := helmSpec.Values[key]; ok {
			helmSpec.Values[key] = toolNS
		}
	}
	if _, ok := helmSpec.Values["fullnameOverride"]; ok {
		helmSpec.Values["fullnameOverride"] = helmSpec.ReleaseName
	}
}

func toolNamespaceValueKeys(helmSpec *paapv1.HelmInstallSpec) []string {
	keys := []string{
		"tool_namespace",
		"paap.toolNamespace",
		"global.paap.toolNamespace",
	}
	if helmSpec == nil || helmSpec.PlatformManifest == "" {
		return keys
	}
	var manifest model.PlatformManifest
	if err := json.Unmarshal([]byte(helmSpec.PlatformManifest), &manifest); err != nil {
		return keys
	}
	seen := make(map[string]bool, len(keys)+len(manifest.VariableMapping))
	for _, key := range keys {
		seen[key] = true
	}
	for _, mapping := range manifest.VariableMapping {
		if mapping.PlatformVar != "tool_namespace" || strings.TrimSpace(mapping.HelmVar) == "" {
			continue
		}
		if !seen[mapping.HelmVar] {
			keys = append(keys, mapping.HelmVar)
			seen[mapping.HelmVar] = true
		}
	}
	return keys
}

func serviceInstanceAnnotations(toolNS, appIdentifier, envIdentifier, svcType string, labels map[string]string) map[string]string {
	annotations := map[string]string{
		"paap.io/tool-namespace":    toolNS,
		"paap.io/service-namespace": toolNS,
		"paap.io/app":               appIdentifier,
		"paap.io/env":               envIdentifier,
		"paap.io/service":           svcType,
		"paap.io/service-type":      svcType,
	}
	for _, key := range []string{"paap.io/tool", "paap.io/category", "paap.io/resource-role"} {
		if labels[key] != "" {
			annotations[key] = labels[key]
		}
	}
	return annotations
}

func serviceInstanceSpecNeedsRefresh(current, next paapv1.ServiceInstanceSpec) bool {
	return !apiequality.Semantic.DeepEqual(current, next)
}

// DeleteServiceInstanceCR deletes a ServiceInstance CR
func DeleteServiceInstanceCR(ctx context.Context, appIdentifier, envIdentifier, svcType string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	svc, err := findServiceInstanceCR(ctx, cl, appIdentifier, envIdentifier, svcType)
	if err != nil {
		return client.IgnoreNotFound(err)
	}
	if svc == nil {
		return nil
	}
	return cl.Delete(ctx, svc)
}

// GetServiceInstanceCRStatus fetches a ServiceInstance CR and returns its status fields.
func GetServiceInstanceCRStatus(ctx context.Context, appIdentifier, envIdentifier, svcType string) (*paapv1.ServiceInstanceStatus, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	svc, err := findServiceInstanceCR(ctx, cl, appIdentifier, envIdentifier, svcType)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, nil
	}
	return &svc.Status, nil
}

func findServiceInstanceCR(ctx context.Context, cl client.Client, appIdentifier, envIdentifier, svcType string) (*paapv1.ServiceInstance, error) {
	namespace := fmt.Sprintf("paap-app-%s", appIdentifier)
	items := &paapv1.ServiceInstanceList{}
	if err := cl.List(ctx, items,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"paap.io/env":          envIdentifier,
			"paap.io/service-type": svcType,
		},
	); err != nil {
		return nil, err
	}
	for i := range items.Items {
		item := &items.Items[i]
		if item.Name == serviceInstanceCRName(envIdentifier, svcType, item.Spec.Helm, item.Labels) {
			return item, nil
		}
	}
	return nil, nil
}

// CreateComponentCR creates a Component CR
func CreateComponentCR(ctx context.Context, appIdentifier, envIdentifier, compName, compIdentifier, compType, image, tag string, replicas int32, targetNamespace, managedBy string, config model.ComponentConfig, envVars ...[]paapv1.EnvVar) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	if managedBy == "" {
		managedBy = "operator"
	}
	comp := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", envIdentifier, compIdentifier),
			Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
			Labels: map[string]string{
				"paap.io/app":       appIdentifier,
				"paap.io/env":       envIdentifier,
				"paap.io/component": compIdentifier,
			},
		},
		Spec: paapv1.ComponentSpec{
			EnvironmentRef: paapv1.ObjectReference{
				Name: envIdentifier,
			},
			Name:         compName,
			Identifier:   compIdentifier,
			Type:         compType,
			ManagedBy:    managedBy,
			ArgoCDAppRef: componentArgoCDAppRef(appIdentifier, envIdentifier, compIdentifier, managedBy),
			Deployment: paapv1.DeploymentSpec{
				Namespace: targetNamespace,
				Image:     image,
				Tag:       tag,
				Replicas:  replicas,
				Command:   config.Command,
				Args:      config.Args,
				Env:       firstComponentEnvVars(envVars),
			},
			Service: componentServiceSpec(compType, config),
		},
	}
	return cl.Create(ctx, comp)
}

// UpsertComponentCR creates the Component CR on first deploy and updates it on later deploys.
func UpsertComponentCR(ctx context.Context, appIdentifier, envIdentifier, compName, compIdentifier, compType, image, tag string, replicas int32, targetNamespace, managedBy string, config model.ComponentConfig, envVars ...[]paapv1.EnvVar) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	if managedBy == "" {
		managedBy = "operator"
	}
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", envIdentifier, compIdentifier),
		Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
	}
	comp := &paapv1.Component{}
	if err := cl.Get(ctx, key, comp); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return CreateComponentCR(ctx, appIdentifier, envIdentifier, compName, compIdentifier, compType, image, tag, replicas, targetNamespace, managedBy, config, firstComponentEnvVars(envVars))
	}
	comp.Spec.Name = compName
	comp.Spec.Identifier = compIdentifier
	comp.Spec.Type = compType
	comp.Spec.ManagedBy = managedBy
	comp.Spec.ArgoCDAppRef = componentArgoCDAppRef(appIdentifier, envIdentifier, compIdentifier, managedBy)
	comp.Spec.Deployment.Namespace = targetNamespace
	comp.Spec.Deployment.Image = image
	comp.Spec.Deployment.Tag = tag
	comp.Spec.Deployment.Replicas = replicas
	comp.Spec.Deployment.Command = config.Command
	comp.Spec.Deployment.Args = config.Args
	comp.Spec.Deployment.Env = firstComponentEnvVars(envVars)
	if comp.Spec.Service == nil {
		comp.Spec.Service = componentServiceSpec(compType, config)
	} else {
		comp.Spec.Service = componentServiceSpec(compType, config)
	}
	return cl.Update(ctx, comp)
}

func componentArgoCDAppRef(appIdentifier, envIdentifier, compIdentifier, managedBy string) *paapv1.ObjectReference {
	if managedBy != "argocd" {
		return nil
	}
	return &paapv1.ObjectReference{Name: fmt.Sprintf("%s-%s-%s", appIdentifier, envIdentifier, compIdentifier)}
}

func componentServiceSpec(compType string, config model.ComponentConfig) *paapv1.ServiceSpec {
	serviceType := "ClusterIP"
	targetPort := model.ResolveComponentContainerPort(compType, config)
	if strings.EqualFold(strings.TrimSpace(compType), "frontend") {
		serviceType = "NodePort"
	}
	return &paapv1.ServiceSpec{Port: 80, TargetPort: targetPort, Type: serviceType}
}

func firstComponentEnvVars(items []([]paapv1.EnvVar)) []paapv1.EnvVar {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

// DeleteComponentCR deletes a Component CR
func DeleteComponentCR(ctx context.Context, appIdentifier, envIdentifier, compIdentifier string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	comp := &paapv1.Component{}
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", envIdentifier, compIdentifier),
		Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
	}
	if err := cl.Get(ctx, key, comp); err != nil {
		return client.IgnoreNotFound(err)
	}
	return cl.Delete(ctx, comp)
}

// DeleteComponentRuntimeResources removes component workloads that may have been
// created by ArgoCD or older operator-managed deployments.
func DeleteComponentRuntimeResources(ctx context.Context, namespace, identifier string, aliases ...string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	namespace = strings.TrimSpace(namespace)
	identifier = strings.TrimSpace(identifier)
	if namespace == "" || identifier == "" {
		return fmt.Errorf("namespace and identifier are required")
	}

	names := uniqueRuntimeCleanupNames(append([]string{identifier, fmt.Sprintf("%s-%s", identifier, namespace)}, aliases...)...)
	for _, name := range names {
		deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		if err := cl.Delete(ctx, deploy); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		stateful := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		if err := cl.Delete(ctx, stateful); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		daemon := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		if err := cl.Delete(ctx, daemon); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		if err := cl.Delete(ctx, svc); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	for _, selector := range componentRuntimeCleanupSelectors(names) {
		if err := deleteComponentDeploymentsByLabels(ctx, cl, namespace, selector); err != nil {
			return err
		}
		if err := deleteComponentStatefulSetsByLabels(ctx, cl, namespace, selector); err != nil {
			return err
		}
		if err := deleteComponentDaemonSetsByLabels(ctx, cl, namespace, selector); err != nil {
			return err
		}
		if err := deleteComponentReplicaSetsByLabels(ctx, cl, namespace, selector); err != nil {
			return err
		}
		if err := deleteComponentPodsByLabels(ctx, cl, namespace, selector); err != nil {
			return err
		}
		if err := deleteComponentServicesByLabels(ctx, cl, namespace, selector); err != nil {
			return err
		}
		if err := deleteComponentIngressesByLabels(ctx, cl, namespace, selector); err != nil {
			return err
		}
	}
	return nil
}

func uniqueRuntimeCleanupNames(names ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.Trim(strings.TrimSpace(name), "/")
		if name == "" {
			continue
		}
		if strings.HasPrefix(name, "components/") {
			name = strings.TrimPrefix(name, "components/")
			if slash := strings.Index(name, "/"); slash >= 0 {
				name = name[:slash]
			}
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func componentRuntimeCleanupSelectors(names []string) []map[string]string {
	selectors := make([]map[string]string, 0, len(names)*4)
	for _, name := range names {
		selectors = append(selectors,
			map[string]string{"paap.io/component": name},
			map[string]string{"app": name},
			map[string]string{"app.kubernetes.io/name": name},
			map[string]string{"app.kubernetes.io/instance": name},
		)
	}
	return selectors
}

func deleteComponentDeploymentsByLabels(ctx context.Context, cl client.Client, namespace string, selector map[string]string) error {
	list := &appsv1.DeploymentList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range list.Items {
		if err := cl.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func deleteComponentStatefulSetsByLabels(ctx context.Context, cl client.Client, namespace string, selector map[string]string) error {
	list := &appsv1.StatefulSetList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range list.Items {
		if err := cl.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func deleteComponentDaemonSetsByLabels(ctx context.Context, cl client.Client, namespace string, selector map[string]string) error {
	list := &appsv1.DaemonSetList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range list.Items {
		if err := cl.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func deleteComponentReplicaSetsByLabels(ctx context.Context, cl client.Client, namespace string, selector map[string]string) error {
	list := &appsv1.ReplicaSetList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range list.Items {
		if err := cl.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func deleteComponentPodsByLabels(ctx context.Context, cl client.Client, namespace string, selector map[string]string) error {
	list := &corev1.PodList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range list.Items {
		if err := cl.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func deleteComponentServicesByLabels(ctx context.Context, cl client.Client, namespace string, selector map[string]string) error {
	list := &corev1.ServiceList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range list.Items {
		if err := cl.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func deleteComponentIngressesByLabels(ctx context.Context, cl client.Client, namespace string, selector map[string]string) error {
	list := &networkingv1.IngressList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range list.Items {
		if err := cl.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// UpdateComponentCR updates the image and tag of a Component CR
func UpdateComponentCR(ctx context.Context, appIdentifier, envIdentifier, compIdentifier, image, tag string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	comp := &paapv1.Component{}
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", envIdentifier, compIdentifier),
		Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
	}
	if err := cl.Get(ctx, key, comp); err != nil {
		return client.IgnoreNotFound(err)
	}
	comp.Spec.Deployment.Image = image
	comp.Spec.Deployment.Tag = tag
	return cl.Update(ctx, comp)
}
