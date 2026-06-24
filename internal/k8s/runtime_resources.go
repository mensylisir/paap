package k8s

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RuntimeResource struct {
	Name        string
	Type        string
	Status      string
	Description string
}

type AdoptableResource struct {
	Key           string         `json:"key"`
	Name          string         `json:"name"`
	Kind          string         `json:"kind"`
	Namespace     string         `json:"namespace"`
	ComponentType string         `json:"componentType"`
	Status        string         `json:"status"`
	Description   string         `json:"description"`
	RuntimeConfig *RuntimeConfig `json:"runtimeConfig,omitempty"`
}

type RuntimeConfig struct {
	Namespace    string                  `json:"namespace,omitempty"`
	WorkloadName string                  `json:"workloadName,omitempty"`
	WorkloadKind string                  `json:"workloadKind,omitempty"`
	ServiceName  string                  `json:"serviceName,omitempty"`
	Container    string                  `json:"container,omitempty"`
	Image        string                  `json:"image,omitempty"`
	Ports        []int32                 `json:"ports,omitempty"`
	Replicas     *int32                  `json:"replicas,omitempty"`
	Command      []string                `json:"command,omitempty"`
	Args         []string                `json:"args,omitempty"`
	Env          []RuntimeEnvVar         `json:"env,omitempty"`
	EnvFrom      []RuntimeEnvFrom        `json:"envFrom,omitempty"`
	Resources    RuntimeResourceRequests `json:"resources,omitempty"`
	Files        []RuntimeConfigFile     `json:"files,omitempty"`
	ConfigMaps   []RuntimeConfigObject   `json:"configMaps,omitempty"`
	Secrets      []RuntimeConfigObject   `json:"secrets,omitempty"`
}

type RuntimeEnvVar struct {
	Name          string `json:"name"`
	Value         string `json:"value,omitempty"`
	SecretName    string `json:"secretName,omitempty"`
	SecretKey     string `json:"secretKey,omitempty"`
	ConfigMapName string `json:"configMapName,omitempty"`
	ConfigMapKey  string `json:"configMapKey,omitempty"`
}

type RuntimeEnvFrom struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type RuntimeResourceRequests struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

type RuntimeConfigFile struct {
	Name       string `json:"name,omitempty"`
	Kind       string `json:"kind,omitempty"`
	ObjectName string `json:"objectName,omitempty"`
	Key        string `json:"key,omitempty"`
	Path       string `json:"path,omitempty"`
	MountPath  string `json:"mountPath,omitempty"`
	ReadOnly   bool   `json:"readOnly,omitempty"`
}

type RuntimeConfigObject struct {
	Name string   `json:"name"`
	Keys []string `json:"keys,omitempty"`
}

func ListNamespaceRuntimeResources(ctx context.Context, namespace string) ([]RuntimeResource, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	resources := make([]RuntimeResource, 0)

	services := &corev1.ServiceList{}
	if err := cl.List(ctx, services, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	for _, svc := range services.Items {
		resources = append(resources, RuntimeResource{
			Name:        svc.Name,
			Type:        "Service",
			Status:      "Ready",
			Description: fmt.Sprintf("%s service, cluster IP %s", svc.Spec.Type, valueOrDash(svc.Spec.ClusterIP)),
		})
	}

	pods := &corev1.PodList{}
	if err := cl.List(ctx, pods, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	for _, pod := range pods.Items {
		resources = append(resources, RuntimeResource{
			Name:        pod.Name,
			Type:        "Pod",
			Status:      string(pod.Status.Phase),
			Description: fmt.Sprintf("Node %s", valueOrDash(pod.Spec.NodeName)),
		})
	}

	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcs, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list pvcs: %w", err)
	}
	for _, pvc := range pvcs.Items {
		resources = append(resources, RuntimeResource{
			Name:        pvc.Name,
			Type:        "Storage",
			Status:      string(pvc.Status.Phase),
			Description: "PersistentVolumeClaim",
		})
	}
	return resources, nil
}

func ListNamespaceAdoptableResources(ctx context.Context, namespace string) ([]AdoptableResource, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, nil
	}
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	out := make([]AdoptableResource, 0)
	add := func(item AdoptableResource) {
		if item.Name == "" || item.Kind == "" || item.Namespace == "" {
			return
		}
		item.Key = strings.ToLower(item.Namespace + "/" + item.Kind + "/" + item.Name)
		if item.ComponentType == "" {
			item.ComponentType = inferAdoptableComponentType(item.Name, item.Kind, item.RuntimeConfig)
		}
		out = append(out, item)
	}

	deployments := &appsv1.DeploymentList{}
	if err := cl.List(ctx, deployments, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list adoptable deployments: %w", err)
	}
	sort.Slice(deployments.Items, func(i, j int) bool { return deployments.Items[i].Name < deployments.Items[j].Name })
	for _, deploy := range deployments.Items {
		if len(deploy.Spec.Template.Spec.Containers) == 0 {
			continue
		}
		cfg := runtimeConfigFromPodTemplate(deploy.Namespace, "Deployment", deploy.Name, deploy.Spec.Replicas, deploy.Spec.Template.Spec, deploy.Spec.Template.Spec.Containers[0])
		enrichRuntimeConfigObjects(ctx, cfg)
		add(AdoptableResource{
			Name:          deploy.Name,
			Kind:          "Deployment",
			Namespace:     deploy.Namespace,
			Status:        deploymentAdoptableStatus(deploy),
			Description:   cfg.Image,
			RuntimeConfig: cfg,
		})
	}

	statefulSets := &appsv1.StatefulSetList{}
	if err := cl.List(ctx, statefulSets, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list adoptable statefulsets: %w", err)
	}
	sort.Slice(statefulSets.Items, func(i, j int) bool { return statefulSets.Items[i].Name < statefulSets.Items[j].Name })
	for _, sts := range statefulSets.Items {
		if len(sts.Spec.Template.Spec.Containers) == 0 {
			continue
		}
		cfg := runtimeConfigFromPodTemplate(sts.Namespace, "StatefulSet", sts.Name, sts.Spec.Replicas, sts.Spec.Template.Spec, sts.Spec.Template.Spec.Containers[0])
		enrichRuntimeConfigObjects(ctx, cfg)
		add(AdoptableResource{
			Name:          sts.Name,
			Kind:          "StatefulSet",
			Namespace:     sts.Namespace,
			ComponentType: "middleware",
			Status:        statefulSetAdoptableStatus(sts),
			Description:   cfg.Image,
			RuntimeConfig: cfg,
		})
	}

	daemonSets := &appsv1.DaemonSetList{}
	if err := cl.List(ctx, daemonSets, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list adoptable daemonsets: %w", err)
	}
	sort.Slice(daemonSets.Items, func(i, j int) bool { return daemonSets.Items[i].Name < daemonSets.Items[j].Name })
	for _, ds := range daemonSets.Items {
		if len(ds.Spec.Template.Spec.Containers) == 0 {
			continue
		}
		cfg := runtimeConfigFromPodTemplate(ds.Namespace, "DaemonSet", ds.Name, nil, ds.Spec.Template.Spec, ds.Spec.Template.Spec.Containers[0])
		enrichRuntimeConfigObjects(ctx, cfg)
		add(AdoptableResource{
			Name:          ds.Name,
			Kind:          "DaemonSet",
			Namespace:     ds.Namespace,
			ComponentType: "middleware",
			Status:        daemonSetAdoptableStatus(ds),
			Description:   cfg.Image,
			RuntimeConfig: cfg,
		})
	}

	return out, nil
}

func inferAdoptableComponentType(name, kind string, cfg *RuntimeConfig) string {
	text := strings.ToLower(strings.TrimSpace(name + " " + kind + " " + valueOrDash(runtimeConfigImage(cfg))))
	switch {
	case strings.Contains(text, "frontend"), strings.Contains(text, "web"), strings.Contains(text, "nginx"):
		return "frontend"
	case strings.Contains(text, "postgres"), strings.Contains(text, "mysql"), strings.Contains(text, "mongo"):
		return "database"
	case strings.Contains(text, "redis"), strings.Contains(text, "rabbit"), strings.Contains(text, "kafka"), strings.Contains(text, "minio"):
		return "middleware"
	default:
		return "backend"
	}
}

func runtimeConfigImage(cfg *RuntimeConfig) string {
	if cfg == nil {
		return ""
	}
	return cfg.Image
}

func deploymentAdoptableStatus(deploy appsv1.Deployment) string {
	replicas := int32(0)
	if deploy.Spec.Replicas != nil {
		replicas = *deploy.Spec.Replicas
	}
	if replicas == 0 {
		return "stopped"
	}
	if deploy.Status.ReadyReplicas >= replicas {
		return "running"
	}
	return "pending"
}

func statefulSetAdoptableStatus(sts appsv1.StatefulSet) string {
	replicas := int32(0)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}
	if replicas == 0 {
		return "stopped"
	}
	if sts.Status.ReadyReplicas >= replicas {
		return "running"
	}
	return "pending"
}

func daemonSetAdoptableStatus(ds appsv1.DaemonSet) string {
	if ds.Status.DesiredNumberScheduled == 0 {
		return "stopped"
	}
	if ds.Status.NumberReady >= ds.Status.DesiredNumberScheduled {
		return "running"
	}
	return "pending"
}

func DiscoverComponentRuntimeConfig(ctx context.Context, namespace, component string) (*RuntimeConfig, error) {
	namespace = strings.TrimSpace(namespace)
	component = strings.TrimSpace(component)
	if namespace == "" || component == "" {
		return nil, nil
	}
	selector := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{"paap.io/component": component},
	}
	if cfg, err := discoverDeploymentRuntimeConfig(ctx, selector...); err == nil && cfg != nil {
		return cfg, nil
	}
	fallback := []client.ListOption{client.InNamespace(namespace), client.MatchingLabels{"app": component}}
	if cfg, err := discoverDeploymentRuntimeConfig(ctx, fallback...); err == nil && cfg != nil {
		return cfg, nil
	}
	return nil, nil
}

func DiscoverNamespaceRuntimeConfig(ctx context.Context, namespace string) (*RuntimeConfig, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, nil
	}
	if cfg, err := discoverDeploymentRuntimeConfig(ctx, client.InNamespace(namespace)); err == nil && cfg != nil {
		return cfg, nil
	}
	if cfg, err := discoverStatefulSetRuntimeConfig(ctx, client.InNamespace(namespace)); err == nil && cfg != nil {
		return cfg, nil
	}
	if cfg, err := discoverDaemonSetRuntimeConfig(ctx, client.InNamespace(namespace)); err == nil && cfg != nil {
		return cfg, nil
	}
	return nil, nil
}

func discoverDeploymentRuntimeConfig(ctx context.Context, opts ...client.ListOption) (*RuntimeConfig, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	list := &appsv1.DeploymentList{}
	if err := cl.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })
	for _, deploy := range list.Items {
		if len(deploy.Spec.Template.Spec.Containers) == 0 {
			continue
		}
		cfg := runtimeConfigFromPodTemplate(deploy.Namespace, "Deployment", deploy.Name, deploy.Spec.Replicas, deploy.Spec.Template.Spec, deploy.Spec.Template.Spec.Containers[0])
		enrichRuntimeConfigObjects(ctx, cfg)
		enrichServiceName(ctx, cfg, deploy.Spec.Selector.MatchLabels)
		return cfg, nil
	}
	return nil, nil
}

func discoverStatefulSetRuntimeConfig(ctx context.Context, opts ...client.ListOption) (*RuntimeConfig, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	list := &appsv1.StatefulSetList{}
	if err := cl.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("list statefulsets: %w", err)
	}
	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })
	for _, sts := range list.Items {
		if len(sts.Spec.Template.Spec.Containers) == 0 {
			continue
		}
		cfg := runtimeConfigFromPodTemplate(sts.Namespace, "StatefulSet", sts.Name, sts.Spec.Replicas, sts.Spec.Template.Spec, sts.Spec.Template.Spec.Containers[0])
		enrichRuntimeConfigObjects(ctx, cfg)
		enrichServiceName(ctx, cfg, sts.Spec.Selector.MatchLabels)
		return cfg, nil
	}
	return nil, nil
}

func discoverDaemonSetRuntimeConfig(ctx context.Context, opts ...client.ListOption) (*RuntimeConfig, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	list := &appsv1.DaemonSetList{}
	if err := cl.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("list daemonsets: %w", err)
	}
	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })
	for _, ds := range list.Items {
		if len(ds.Spec.Template.Spec.Containers) == 0 {
			continue
		}
		cfg := runtimeConfigFromPodTemplate(ds.Namespace, "DaemonSet", ds.Name, nil, ds.Spec.Template.Spec, ds.Spec.Template.Spec.Containers[0])
		enrichRuntimeConfigObjects(ctx, cfg)
		return cfg, nil
	}
	return nil, nil
}

func runtimeConfigFromPodTemplate(namespace, kind, workloadName string, replicas *int32, podSpec corev1.PodSpec, container corev1.Container) *RuntimeConfig {
	cfg := &RuntimeConfig{
		Namespace:    namespace,
		WorkloadName: workloadName,
		WorkloadKind: kind,
		Container:    container.Name,
		Image:        container.Image,
		Ports:        runtimeContainerPorts(container.Ports),
		Replicas:     replicas,
		Command:      trimStringSlice(container.Command),
		Args:         trimStringSlice(container.Args),
		Env:          runtimeEnvVars(container.Env),
		EnvFrom:      runtimeEnvFrom(container.EnvFrom),
		Files:        runtimeConfigFiles(podSpec, container),
		Resources: RuntimeResourceRequests{
			Requests: resourceListToStrings(container.Resources.Requests),
			Limits:   resourceListToStrings(container.Resources.Limits),
		},
	}
	return cfg
}

func enrichServiceName(ctx context.Context, cfg *RuntimeConfig, matchLabels map[string]string) {
	if cfg == nil || cfg.Namespace == "" || len(matchLabels) == 0 {
		return
	}
	cl, err := requireClient()
	if err != nil {
		return
	}
	svcs := &corev1.ServiceList{}
	if err := cl.List(ctx, svcs, client.InNamespace(cfg.Namespace)); err != nil {
		return
	}
	for _, svc := range svcs.Items {
		if svc.Spec.Selector == nil {
			continue
		}
		matches := true
		for k, v := range matchLabels {
			if svc.Spec.Selector[k] != v {
				matches = false
				break
			}
		}
		if matches {
			cfg.ServiceName = svc.Name
			return
		}
	}
}

func runtimeContainerPorts(ports []corev1.ContainerPort) []int32 {
	if len(ports) == 0 {
		return nil
	}
	out := make([]int32, 0, len(ports))
	seen := map[int32]struct{}{}
	for _, port := range ports {
		if port.ContainerPort <= 0 {
			continue
		}
		if _, exists := seen[port.ContainerPort]; exists {
			continue
		}
		seen[port.ContainerPort] = struct{}{}
		out = append(out, port.ContainerPort)
	}
	return out
}

func runtimeConfigFiles(podSpec corev1.PodSpec, container corev1.Container) []RuntimeConfigFile {
	if len(container.VolumeMounts) == 0 || len(podSpec.Volumes) == 0 {
		return nil
	}
	volumes := map[string]corev1.Volume{}
	for _, volume := range podSpec.Volumes {
		if volume.Name != "" {
			volumes[volume.Name] = volume
		}
	}
	out := make([]RuntimeConfigFile, 0)
	add := func(mount corev1.VolumeMount, kind, objectName string, items []corev1.KeyToPath) {
		objectName = strings.TrimSpace(objectName)
		if objectName == "" || strings.TrimSpace(mount.MountPath) == "" {
			return
		}
		if len(items) == 0 {
			item := RuntimeConfigFile{
				Name:       mount.Name,
				Kind:       kind,
				ObjectName: objectName,
				MountPath:  strings.TrimSpace(mount.MountPath),
				ReadOnly:   mount.ReadOnly,
			}
			if mount.SubPath != "" {
				item.Key = strings.TrimSpace(mount.SubPath)
				item.Path = strings.TrimSpace(mount.SubPath)
			}
			out = append(out, item)
			return
		}
		for _, volumeItem := range items {
			key := strings.TrimSpace(volumeItem.Key)
			relPath := strings.TrimSpace(volumeItem.Path)
			mountPath := strings.TrimSpace(mount.MountPath)
			if mount.SubPath == "" && relPath != "" {
				mountPath = path.Join(mountPath, relPath)
			}
			out = append(out, RuntimeConfigFile{
				Name:       mount.Name,
				Kind:       kind,
				ObjectName: objectName,
				Key:        key,
				Path:       relPath,
				MountPath:  mountPath,
				ReadOnly:   mount.ReadOnly,
			})
		}
	}

	for _, mount := range container.VolumeMounts {
		volume, ok := volumes[mount.Name]
		if !ok {
			continue
		}
		if volume.ConfigMap != nil {
			add(mount, "configMap", volume.ConfigMap.Name, volume.ConfigMap.Items)
		}
		if volume.Secret != nil {
			add(mount, "secret", volume.Secret.SecretName, volume.Secret.Items)
		}
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MountPath != out[j].MountPath {
			return out[i].MountPath < out[j].MountPath
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].ObjectName != out[j].ObjectName {
			return out[i].ObjectName < out[j].ObjectName
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func runtimeEnvVars(items []corev1.EnvVar) []RuntimeEnvVar {
	out := make([]RuntimeEnvVar, 0, len(items))
	for _, item := range items {
		env := RuntimeEnvVar{Name: item.Name, Value: item.Value}
		if item.ValueFrom != nil && item.ValueFrom.SecretKeyRef != nil {
			env.Value = ""
			env.SecretName = item.ValueFrom.SecretKeyRef.Name
			env.SecretKey = item.ValueFrom.SecretKeyRef.Key
		}
		if item.ValueFrom != nil && item.ValueFrom.ConfigMapKeyRef != nil {
			env.Value = ""
			env.ConfigMapName = item.ValueFrom.ConfigMapKeyRef.Name
			env.ConfigMapKey = item.ValueFrom.ConfigMapKeyRef.Key
		}
		out = append(out, env)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func runtimeEnvFrom(items []corev1.EnvFromSource) []RuntimeEnvFrom {
	out := make([]RuntimeEnvFrom, 0, len(items))
	for _, item := range items {
		if item.ConfigMapRef != nil && item.ConfigMapRef.Name != "" {
			out = append(out, RuntimeEnvFrom{Kind: "configMap", Name: item.ConfigMapRef.Name})
		}
		if item.SecretRef != nil && item.SecretRef.Name != "" {
			out = append(out, RuntimeEnvFrom{Kind: "secret", Name: item.SecretRef.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func enrichRuntimeConfigObjects(ctx context.Context, cfg *RuntimeConfig) {
	if cfg == nil || cfg.Namespace == "" {
		return
	}
	cfg.ConfigMaps = discoverRuntimeConfigMaps(ctx, cfg.Namespace, cfg)
	cfg.Secrets = discoverRuntimeSecrets(ctx, cfg.Namespace, cfg)
}

func discoverRuntimeConfigMaps(ctx context.Context, namespace string, cfg *RuntimeConfig) []RuntimeConfigObject {
	names := map[string]struct{}{}
	for _, env := range cfg.Env {
		if env.ConfigMapName != "" {
			names[env.ConfigMapName] = struct{}{}
		}
	}
	for _, from := range cfg.EnvFrom {
		if from.Kind == "configMap" && from.Name != "" {
			names[from.Name] = struct{}{}
		}
	}
	if len(names) == 0 {
		return nil
	}
	cl, err := requireClient()
	if err != nil {
		return nil
	}
	out := make([]RuntimeConfigObject, 0, len(names))
	for name := range names {
		cm := &corev1.ConfigMap{}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm); err != nil {
			out = append(out, RuntimeConfigObject{Name: name})
			continue
		}
		keys := make([]string, 0, len(cm.Data))
		for key := range cm.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out = append(out, RuntimeConfigObject{Name: name, Keys: keys})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func discoverRuntimeSecrets(ctx context.Context, namespace string, cfg *RuntimeConfig) []RuntimeConfigObject {
	names := map[string]struct{}{}
	for _, env := range cfg.Env {
		if env.SecretName != "" {
			names[env.SecretName] = struct{}{}
		}
	}
	for _, from := range cfg.EnvFrom {
		if from.Kind == "secret" && from.Name != "" {
			names[from.Name] = struct{}{}
		}
	}
	if len(names) == 0 {
		return nil
	}
	cl, err := requireClient()
	if err != nil {
		return nil
	}
	out := make([]RuntimeConfigObject, 0, len(names))
	for name := range names {
		secret := &corev1.Secret{}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret); err != nil {
			out = append(out, RuntimeConfigObject{Name: name})
			continue
		}
		keys := make([]string, 0, len(secret.Data))
		for key := range secret.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out = append(out, RuntimeConfigObject{Name: name, Keys: keys})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func trimStringSlice(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func resourceListToStrings(items corev1.ResourceList) map[string]string {
	if len(items) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range items {
		if value.Cmp(resource.Quantity{}) != 0 {
			out[string(key)] = value.String()
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
