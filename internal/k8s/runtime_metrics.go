package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RuntimeMetricsTarget struct {
	Namespace    string `json:"namespace"`
	WorkloadName string `json:"workloadName,omitempty"`
	WorkloadKind string `json:"workloadKind,omitempty"`
	Pod          string `json:"pod,omitempty"`
	Container    string `json:"container,omitempty"`
}

type RuntimeMetricSample struct {
	Pod         string  `json:"pod"`
	Container   string  `json:"container"`
	Status      string  `json:"status"`
	CPU         string  `json:"cpu"`
	CPUCores    float64 `json:"cpuCores"`
	Memory      string  `json:"memory"`
	MemoryBytes int64   `json:"memoryBytes"`
	Restarts    int32   `json:"restarts"`
}

type RuntimeMetricsSummary struct {
	Pods        int     `json:"pods"`
	Containers  int     `json:"containers"`
	CPU         string  `json:"cpu"`
	CPUCores    float64 `json:"cpuCores"`
	Memory      string  `json:"memory"`
	MemoryBytes int64   `json:"memoryBytes"`
	Restarts    int32   `json:"restarts"`
}

type RuntimeMetrics struct {
	Target    RuntimeMetricsTarget  `json:"target"`
	Available bool                  `json:"available"`
	Error     string                `json:"error,omitempty"`
	Summary   RuntimeMetricsSummary `json:"summary"`
	Samples   []RuntimeMetricSample `json:"samples"`
	UpdatedAt string                `json:"updatedAt"`
}

type podMetricsList struct {
	Items []podMetricsItem `json:"items"`
}

type podMetricsItem struct {
	Metadata   metav1.ObjectMeta      `json:"metadata"`
	Containers []containerMetricsItem `json:"containers"`
}

type containerMetricsItem struct {
	Name  string            `json:"name"`
	Usage map[string]string `json:"usage"`
}

func GetRuntimeMetrics(ctx context.Context, namespace string, cfg *RuntimeConfig) (RuntimeMetrics, error) {
	namespace = strings.TrimSpace(namespace)
	out := RuntimeMetrics{
		Target: RuntimeMetricsTarget{
			Namespace: namespace,
		},
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if cfg != nil {
		out.Target.WorkloadName = cfg.WorkloadName
		out.Target.WorkloadKind = cfg.WorkloadKind
		out.Target.Container = cfg.Container
		if namespace == "" {
			namespace = cfg.Namespace
			out.Target.Namespace = cfg.Namespace
		}
	}
	if namespace == "" {
		return out, fmt.Errorf("namespace is required")
	}

	pods, err := podsForRuntimeConfig(ctx, namespace, cfg)
	if err != nil {
		return out, err
	}
	out.Summary.Pods = len(pods)
	if len(pods) > 0 {
		out.Target.Pod = pods[0].Name
	}

	podMetrics, metricsErr := readPodMetrics(ctx, namespace)
	if metricsErr != nil {
		out.Available = false
		out.Error = metricsErr.Error()
		out.Samples = samplesFromPodsWithoutUsage(pods, cfg)
		out.Summary.Containers = len(out.Samples)
		out.Summary.Restarts = sumRestarts(out.Samples)
		return out, nil
	}

	metricsByPod := map[string]podMetricsItem{}
	for _, item := range podMetrics.Items {
		metricsByPod[item.Metadata.Name] = item
	}
	samples := make([]RuntimeMetricSample, 0)
	for _, pod := range pods {
		podMetric := metricsByPod[pod.Name]
		containers := containersForMetrics(pod, cfg)
		for _, container := range containers {
			sample := RuntimeMetricSample{
				Pod:       pod.Name,
				Container: container.Name,
				Status:    string(pod.Status.Phase),
				Restarts:  containerRestarts(pod, container.Name),
			}
			if metric, ok := findContainerMetrics(podMetric, container.Name); ok {
				sample.CPU, sample.CPUCores = parseCPUUsage(metric.Usage["cpu"])
				sample.Memory, sample.MemoryBytes = parseMemoryUsage(metric.Usage["memory"])
			}
			samples = append(samples, sample)
		}
	}
	sort.Slice(samples, func(i, j int) bool {
		if samples[i].Pod != samples[j].Pod {
			return samples[i].Pod < samples[j].Pod
		}
		return samples[i].Container < samples[j].Container
	})
	out.Available = true
	out.Samples = samples
	out.Summary = summarizeRuntimeMetrics(out.Summary.Pods, samples)
	return out, nil
}

func ResolveRuntimeTarget(ctx context.Context, namespace string, cfg *RuntimeConfig) (RuntimeMetricsTarget, error) {
	namespace = strings.TrimSpace(namespace)
	if cfg != nil && namespace == "" {
		namespace = cfg.Namespace
	}
	target := RuntimeMetricsTarget{Namespace: namespace}
	if cfg != nil {
		target.WorkloadName = cfg.WorkloadName
		target.WorkloadKind = cfg.WorkloadKind
		target.Container = cfg.Container
	}
	if namespace == "" {
		return target, fmt.Errorf("namespace is required")
	}
	pods, err := podsForRuntimeConfig(ctx, namespace, cfg)
	if err != nil {
		return target, err
	}
	if len(pods) == 0 {
		return target, fmt.Errorf("no pods found for runtime target")
	}
	pod := pods[0]
	target.Pod = pod.Name
	if target.Container == "" && len(pod.Spec.Containers) > 0 {
		target.Container = pod.Spec.Containers[0].Name
	}
	if target.Container == "" {
		return target, fmt.Errorf("no container found for pod %s", pod.Name)
	}
	return target, nil
}

func EnrichRuntimeMetricsFromPrometheus(ctx context.Context, metrics RuntimeMetrics, monitorNamespace string) RuntimeMetrics {
	monitorNamespace = strings.TrimSpace(monitorNamespace)
	if monitorNamespace == "" || metrics.Target.Namespace == "" || len(metrics.Samples) == 0 {
		return metrics
	}
	podPattern := prometheusPodPattern(metrics.Samples)
	if podPattern == "" {
		return metrics
	}
	client := NewPrometheusClient(monitorNamespace)
	namespace := metrics.Target.Namespace
	cpuQuery := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace=%q,pod=~%q,container!="POD",container!=""}[5m])) by (pod,container)`, namespace, podPattern)
	memoryQuery := fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace=%q,pod=~%q,container!="POD",container!=""}) by (pod,container)`, namespace, podPattern)
	cpuSamples, cpuErr := client.Query(ctx, cpuQuery)
	memorySamples, memoryErr := client.Query(ctx, memoryQuery)
	if cpuErr != nil && memoryErr != nil {
		if !metrics.Available && metrics.Error != "" {
			metrics.Error = metrics.Error + "; prometheus unavailable: " + cpuErr.Error()
		} else if !metrics.Available {
			metrics.Error = "prometheus unavailable: " + cpuErr.Error()
		}
		return metrics
	}
	byKey := make(map[string]int, len(metrics.Samples))
	for i, sample := range metrics.Samples {
		byKey[runtimeMetricSampleKey(sample.Pod, sample.Container)] = i
	}
	for _, sample := range cpuSamples {
		idx, ok := byKey[runtimeMetricSampleKey(sample.Metric["pod"], sample.Metric["container"])]
		if !ok {
			continue
		}
		metrics.Samples[idx].CPUCores = sample.Value
		metrics.Samples[idx].CPU = formatCPUCores(sample.Value)
	}
	for _, sample := range memorySamples {
		idx, ok := byKey[runtimeMetricSampleKey(sample.Metric["pod"], sample.Metric["container"])]
		if !ok {
			continue
		}
		bytes := int64(sample.Value)
		metrics.Samples[idx].MemoryBytes = bytes
		metrics.Samples[idx].Memory = formatBytes(bytes)
	}
	if len(cpuSamples) > 0 || len(memorySamples) > 0 {
		metrics.Available = true
		metrics.Error = ""
		metrics.Summary = summarizeRuntimeMetrics(metrics.Summary.Pods, metrics.Samples)
	}
	return metrics
}

func prometheusPodPattern(samples []RuntimeMetricSample) string {
	seen := map[string]struct{}{}
	parts := make([]string, 0, len(samples))
	for _, sample := range samples {
		pod := strings.TrimSpace(sample.Pod)
		if pod == "" {
			continue
		}
		if _, ok := seen[pod]; ok {
			continue
		}
		seen[pod] = struct{}{}
		parts = append(parts, regexp.QuoteMeta(pod))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func runtimeMetricSampleKey(pod, container string) string {
	return strings.TrimSpace(pod) + "\x00" + strings.TrimSpace(container)
}

func podsForRuntimeConfig(ctx context.Context, namespace string, cfg *RuntimeConfig) ([]corev1.Pod, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	opts := []client.ListOption{client.InNamespace(namespace)}
	if cfg != nil && cfg.WorkloadKind != "" && cfg.WorkloadName != "" {
		if selector := workloadSelector(ctx, cl, namespace, cfg.WorkloadKind, cfg.WorkloadName); selector != nil {
			opts = append(opts, client.MatchingLabelsSelector{Selector: selector})
		}
	}
	list := &corev1.PodList{}
	if err := cl.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("list runtime pods: %w", err)
	}
	pods := append([]corev1.Pod(nil), list.Items...)
	if cfg != nil && cfg.WorkloadName != "" {
		filtered := pods[:0]
		for _, pod := range pods {
			if podBelongsToWorkload(pod, cfg.WorkloadName) {
				filtered = append(filtered, pod)
			}
		}
		if len(filtered) > 0 {
			pods = filtered
		}
	}
	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Status.Phase != pods[j].Status.Phase {
			return pods[i].Status.Phase == corev1.PodRunning
		}
		return pods[i].Name < pods[j].Name
	})
	return pods, nil
}

func workloadSelector(ctx context.Context, cl client.Client, namespace, kind, name string) labels.Selector {
	selector := labels.Everything()
	switch strings.ToLower(kind) {
	case "deployment":
		var obj appsv1.Deployment
		if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &obj); err == nil && obj.Spec.Selector != nil {
			if parsed, err := metav1.LabelSelectorAsSelector(obj.Spec.Selector); err == nil {
				return parsed
			}
		}
	case "statefulset":
		var obj appsv1.StatefulSet
		if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &obj); err == nil && obj.Spec.Selector != nil {
			if parsed, err := metav1.LabelSelectorAsSelector(obj.Spec.Selector); err == nil {
				return parsed
			}
		}
	case "daemonset":
		var obj appsv1.DaemonSet
		if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &obj); err == nil && obj.Spec.Selector != nil {
			if parsed, err := metav1.LabelSelectorAsSelector(obj.Spec.Selector); err == nil {
				return parsed
			}
		}
	}
	return selector
}

func podBelongsToWorkload(pod corev1.Pod, workload string) bool {
	workload = strings.TrimSpace(workload)
	if workload == "" {
		return true
	}
	if strings.HasPrefix(pod.Name, workload+"-") || pod.Labels["app.kubernetes.io/instance"] == workload || pod.Labels["app"] == workload {
		return true
	}
	for _, owner := range pod.OwnerReferences {
		if owner.Name == workload || strings.HasPrefix(owner.Name, workload+"-") {
			return true
		}
	}
	return false
}

func readPodMetrics(ctx context.Context, namespace string) (podMetricsList, error) {
	var out podMetricsList
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return out, fmt.Errorf("metrics API config unavailable: %w", err)
	}
	metricsCfg := rest.CopyConfig(cfg)
	metricsCfg.APIPath = "/apis"
	metricsCfg.GroupVersion = &schema.GroupVersion{Group: "metrics.k8s.io", Version: "v1beta1"}
	metricsCfg.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(runtime.NewScheme())}
	client, err := rest.RESTClientFor(metricsCfg)
	if err != nil {
		return out, fmt.Errorf("metrics API client unavailable: %w", err)
	}
	raw, err := client.Get().Namespace(namespace).Resource("pods").DoRaw(ctx)
	if err != nil {
		return out, fmt.Errorf("metrics API unavailable: %w", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode metrics API response: %w", err)
	}
	return out, nil
}

func containersForMetrics(pod corev1.Pod, cfg *RuntimeConfig) []corev1.Container {
	if cfg != nil && cfg.Container != "" {
		for _, container := range pod.Spec.Containers {
			if container.Name == cfg.Container {
				return []corev1.Container{container}
			}
		}
	}
	return append([]corev1.Container(nil), pod.Spec.Containers...)
}

func samplesFromPodsWithoutUsage(pods []corev1.Pod, cfg *RuntimeConfig) []RuntimeMetricSample {
	samples := make([]RuntimeMetricSample, 0)
	for _, pod := range pods {
		for _, container := range containersForMetrics(pod, cfg) {
			samples = append(samples, RuntimeMetricSample{
				Pod:       pod.Name,
				Container: container.Name,
				Status:    string(pod.Status.Phase),
				Restarts:  containerRestarts(pod, container.Name),
			})
		}
	}
	return samples
}

func findContainerMetrics(pod podMetricsItem, container string) (containerMetricsItem, bool) {
	for _, item := range pod.Containers {
		if item.Name == container {
			return item, true
		}
	}
	return containerMetricsItem{}, false
}

func containerRestarts(pod corev1.Pod, container string) int32 {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == container {
			return status.RestartCount
		}
	}
	return 0
}

func parseCPUUsage(raw string) (string, float64) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-", 0
	}
	q, err := resource.ParseQuantity(raw)
	if err != nil {
		return raw, 0
	}
	cores := float64(q.MilliValue()) / 1000
	return formatCPUCores(cores), cores
}

func parseMemoryUsage(raw string) (string, int64) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-", 0
	}
	q, err := resource.ParseQuantity(raw)
	if err != nil {
		return raw, 0
	}
	bytes := q.Value()
	return formatBytes(bytes), bytes
}

func summarizeRuntimeMetrics(pods int, samples []RuntimeMetricSample) RuntimeMetricsSummary {
	var cpu float64
	var memory int64
	var restarts int32
	for _, sample := range samples {
		cpu += sample.CPUCores
		memory += sample.MemoryBytes
		restarts += sample.Restarts
	}
	return RuntimeMetricsSummary{
		Pods:        pods,
		Containers:  len(samples),
		CPU:         formatCPUCores(cpu),
		CPUCores:    cpu,
		Memory:      formatBytes(memory),
		MemoryBytes: memory,
		Restarts:    restarts,
	}
}

func sumRestarts(samples []RuntimeMetricSample) int32 {
	var restarts int32
	for _, sample := range samples {
		restarts += sample.Restarts
	}
	return restarts
}

func formatCPUCores(cores float64) string {
	if cores <= 0 {
		return "0m"
	}
	milli := cores * 1000
	if milli < 1000 {
		return fmt.Sprintf("%.0fm", milli)
	}
	return fmt.Sprintf("%.2f cores", cores)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes <= 0 {
		return "0 B"
	}
	value := float64(bytes)
	for _, suffix := range []string{"B", "KiB", "MiB", "GiB", "TiB"} {
		if value < unit || suffix == "TiB" {
			if suffix == "B" {
				return fmt.Sprintf("%d %s", bytes, suffix)
			}
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
		value /= unit
	}
	return fmt.Sprintf("%d B", bytes)
}
