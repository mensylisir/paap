package k8s

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KubeVirtServiceResourceInput struct {
	AppIdentifier string
	EnvIdentifier string
	ServiceType   string
	ServiceName   string
	Namespace     string
	RuntimeSpec   string
	Labels        map[string]string
	Annotations   map[string]string
}

type KubeVirtServiceResources struct {
	VirtualMachine   *unstructured.Unstructured
	DataVolume       *unstructured.Unstructured
	Service          *corev1.Service
	Secret           *corev1.Secret
	Connections      []KubeVirtConnectionOutput
	MonitoringTarget string
}

type KubeVirtConnectionOutput struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	Host              string `json:"host"`
	Port              int32  `json:"port"`
	SecretName        string `json:"secretName"`
	UsernameSecretKey string `json:"usernameSecretKey,omitempty"`
	PasswordSecretKey string `json:"passwordSecretKey,omitempty"`
	URI               string `json:"uri,omitempty"`
}

type KubeVirtServiceStatus struct {
	Phase   string
	Message string
}

type kubeVirtRuntimeSpec struct {
	VMName            string                       `json:"vmName"`
	Image             string                       `json:"image"`
	CPU               string                       `json:"cpu"`
	Memory            string                       `json:"memory"`
	DiskSize          string                       `json:"diskSize"`
	StorageClassName  string                       `json:"storageClassName"`
	DataVolumeName    string                       `json:"dataVolumeName"`
	SecretName        string                       `json:"secretName"`
	CloudInitUserData string                       `json:"cloudInitUserData"`
	Ports             []kubeVirtRuntimeServicePort `json:"ports"`
	Credentials       map[string]string            `json:"credentials"`
	Database          string                       `json:"database"`
	Readiness         *kubeVirtRuntimeProbe        `json:"readiness"`
	Monitoring        *kubeVirtRuntimeMonitoring   `json:"monitoring"`
	BackupPolicy      *kubeVirtRuntimeBackupPolicy `json:"backupPolicy"`
}

type kubeVirtRuntimeServicePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

type kubeVirtRuntimeProbe struct {
	Type                string   `json:"type"`
	Path                string   `json:"path"`
	Port                int32    `json:"port"`
	Command             []string `json:"command"`
	InitialDelaySeconds int32    `json:"initialDelaySeconds"`
	PeriodSeconds       int32    `json:"periodSeconds"`
	TimeoutSeconds      int32    `json:"timeoutSeconds"`
	FailureThreshold    int32    `json:"failureThreshold"`
}

type kubeVirtRuntimeMonitoring struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
	Port    int32  `json:"port"`
	Scheme  string `json:"scheme"`
}

type kubeVirtRuntimeBackupPolicy struct {
	Enabled       bool   `json:"enabled"`
	Schedule      string `json:"schedule"`
	Retention     string `json:"retention"`
	SnapshotClass string `json:"snapshotClass"`
}

var kubeVirtNameInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)

func ValidateKubeVirtRuntimeSpec(runtimeSpec string) error {
	_, err := parseKubeVirtRuntimeSpec(runtimeSpec)
	return err
}

func BuildKubeVirtServiceResources(input KubeVirtServiceResourceInput) (KubeVirtServiceResources, error) {
	appIdentifier := strings.TrimSpace(input.AppIdentifier)
	envIdentifier := strings.TrimSpace(input.EnvIdentifier)
	serviceType := strings.TrimSpace(input.ServiceType)
	namespace := strings.TrimSpace(input.Namespace)
	if appIdentifier == "" {
		return KubeVirtServiceResources{}, fmt.Errorf("app identifier is required")
	}
	if envIdentifier == "" {
		return KubeVirtServiceResources{}, fmt.Errorf("environment identifier is required")
	}
	if serviceType == "" {
		return KubeVirtServiceResources{}, fmt.Errorf("service type is required")
	}
	if namespace == "" {
		return KubeVirtServiceResources{}, fmt.Errorf("namespace is required")
	}

	spec, err := parseKubeVirtRuntimeSpec(input.RuntimeSpec)
	if err != nil {
		return KubeVirtServiceResources{}, err
	}
	ports, _ := normalizeKubeVirtPorts(spec.Ports)

	vmName := kubeVirtDNSLabel(kvFirstNonEmpty(spec.VMName, input.ServiceName, serviceType), "service")
	secretName := kubeVirtDNSLabel(kvFirstNonEmpty(spec.SecretName, vmName+"-credentials"), vmName+"-credentials")
	labels := kubeVirtServiceLabels(input, vmName)
	annotations := kvCopyStringMap(input.Annotations)
	annotations = kubeVirtRuntimeAnnotations(spec, annotations)
	credentials, err := kubeVirtCredentials(serviceType, spec.Credentials)
	if err != nil {
		return KubeVirtServiceResources{}, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{},
	}
	for key, value := range credentials {
		secret.Data[key] = []byte(value)
	}

	vm, dataVolume := kubeVirtVirtualMachine(namespace, vmName, spec, labels, annotations)
	k8sService := kubeVirtService(namespace, vmName, serviceType, ports, labels, annotations)
	connections := kubeVirtConnections(serviceType, namespace, k8sService.Name, ports, secretName, credentials, spec.Database)

	return KubeVirtServiceResources{
		VirtualMachine:   vm,
		DataVolume:       dataVolume,
		Service:          k8sService,
		Secret:           secret,
		Connections:      connections,
		MonitoringTarget: kubeVirtMonitoringTarget(namespace, serviceType, spec),
	}, nil
}

func parseKubeVirtRuntimeSpec(runtimeSpec string) (kubeVirtRuntimeSpec, error) {
	var spec kubeVirtRuntimeSpec
	if err := json.Unmarshal([]byte(strings.TrimSpace(runtimeSpec)), &spec); err != nil {
		return kubeVirtRuntimeSpec{}, fmt.Errorf("parse kubevirt runtime spec: %w", err)
	}
	spec.Image = strings.TrimSpace(spec.Image)
	if spec.Image == "" {
		return kubeVirtRuntimeSpec{}, fmt.Errorf("kubevirt runtime spec image is required")
	}
	if _, err := normalizeKubeVirtPorts(spec.Ports); err != nil {
		return kubeVirtRuntimeSpec{}, err
	}
	if err := validateKubeVirtProbe(spec.Readiness); err != nil {
		return kubeVirtRuntimeSpec{}, err
	}
	if err := validateKubeVirtMonitoring(spec.Monitoring); err != nil {
		return kubeVirtRuntimeSpec{}, err
	}
	return spec, nil
}

func UpsertKubeVirtServiceResources(ctx context.Context, resources KubeVirtServiceResources) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	if resources.Service == nil {
		return fmt.Errorf("kubevirt service resource is required")
	}
	if resources.Secret == nil {
		return fmt.Errorf("kubevirt secret resource is required")
	}
	if resources.VirtualMachine == nil {
		return fmt.Errorf("kubevirt virtual machine resource is required")
	}
	namespace := strings.TrimSpace(resources.Service.Namespace)
	if namespace == "" {
		return fmt.Errorf("kubevirt namespace is required")
	}
	if err := upsertKubeVirtNamespace(ctx, cl, namespace, resources.Service.Labels); err != nil {
		return err
	}
	if err := upsertKubeVirtSecret(ctx, cl, resources.Secret); err != nil {
		return err
	}
	if err := upsertKubeVirtService(ctx, cl, resources.Service); err != nil {
		return err
	}
	if resources.DataVolume != nil {
		if err := upsertKubeVirtUnstructured(ctx, cl, resources.DataVolume); err != nil {
			return err
		}
	}
	if err := upsertKubeVirtUnstructured(ctx, cl, resources.VirtualMachine); err != nil {
		return err
	}
	return nil
}

func DeleteKubeVirtServiceResources(ctx context.Context, namespace string, serviceType string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	namespace = strings.TrimSpace(namespace)
	serviceType = strings.TrimSpace(serviceType)
	if namespace == "" {
		return fmt.Errorf("kubevirt namespace is required")
	}
	if serviceType == "" {
		return fmt.Errorf("service type is required")
	}
	labels := client.MatchingLabels{
		"paap.io/provision-mode": "kubevirt",
		"paap.io/service-type":   serviceType,
	}
	if err := deleteKubeVirtUnstructuredList(ctx, cl, "kubevirt.io/v1", "VirtualMachineList", namespace, labels); err != nil {
		return err
	}
	if err := deleteKubeVirtUnstructuredList(ctx, cl, "cdi.kubevirt.io/v1beta1", "DataVolumeList", namespace, labels); err != nil {
		return err
	}
	if err := deleteKubeVirtServices(ctx, cl, namespace, labels); err != nil {
		return err
	}
	if err := deleteKubeVirtSecrets(ctx, cl, namespace, labels); err != nil {
		return err
	}
	ns := &corev1.Namespace{}
	if err := cl.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
		return client.IgnoreNotFound(err)
	}
	return client.IgnoreNotFound(cl.Delete(ctx, ns))
}

func DiscoverKubeVirtServiceRuntimeConfig(ctx context.Context, namespace string, serviceType string) (*RuntimeConfig, error) {
	vm, err := getKubeVirtServiceVirtualMachine(ctx, namespace, serviceType)
	if err != nil || vm == nil {
		return nil, err
	}
	cfg := &RuntimeConfig{
		Namespace:    strings.TrimSpace(namespace),
		WorkloadName: vm.GetName(),
		WorkloadKind: "VirtualMachine",
		Image:        kubeVirtRuntimeImage(ctx, namespace, serviceType, vm),
		Resources: RuntimeResourceRequests{
			Requests: kubeVirtRuntimeResourceRequests(vm),
		},
		Ports: kubeVirtRuntimePorts(ctx, namespace, serviceType),
	}
	if network, err := DiscoverNamespaceServiceNetwork(ctx, namespace, serviceType); err == nil && network != nil {
		cfg.ServiceName = network.ServiceName
	}
	if secretObjects := kubeVirtRuntimeSecrets(ctx, namespace, serviceType); len(secretObjects) > 0 {
		cfg.Secrets = secretObjects
	}
	return cfg, nil
}

func DiscoverKubeVirtServiceStatus(ctx context.Context, namespace string, serviceType string) (*KubeVirtServiceStatus, error) {
	vm, err := getKubeVirtServiceVirtualMachine(ctx, namespace, serviceType)
	if err != nil || vm == nil {
		return nil, err
	}
	printable, _, _ := unstructured.NestedString(vm.Object, "status", "printableStatus")
	phase := kubeVirtPrintableStatusPhase(printable)
	message := strings.TrimSpace(printable)
	if phase == "" {
		if ready, readyMessage := kubeVirtReadyCondition(vm); ready {
			phase = "running"
			message = readyMessage
		} else if readyMessage != "" {
			phase = "installing"
			message = readyMessage
		}
	}
	if phase == "" {
		phase = "installing"
	}
	return &KubeVirtServiceStatus{Phase: phase, Message: message}, nil
}

func getKubeVirtServiceVirtualMachine(ctx context.Context, namespace string, serviceType string) (*unstructured.Unstructured, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	namespace = strings.TrimSpace(namespace)
	serviceType = strings.TrimSpace(serviceType)
	if namespace == "" || serviceType == "" {
		return nil, nil
	}
	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion("kubevirt.io/v1")
	list.SetKind("VirtualMachineList")
	if err := cl.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"paap.io/provision-mode": "kubevirt",
			"paap.io/service-type":   serviceType,
		},
	); err != nil {
		return nil, fmt.Errorf("list kubevirt virtual machines in %s: %w", namespace, err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].GetName() < list.Items[j].GetName() })
	return &list.Items[0], nil
}

func kubeVirtRuntimeImage(ctx context.Context, namespace, serviceType string, vm *unstructured.Unstructured) string {
	volumes, _, _ := unstructured.NestedSlice(vm.Object, "spec", "template", "spec", "volumes")
	for _, raw := range volumes {
		volume, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if image, _, _ := unstructured.NestedString(volume, "containerDisk", "image"); image != "" {
			return image
		}
	}
	if image := kubeVirtDataVolumeImage(ctx, namespace, serviceType); image != "" {
		return image
	}
	return ""
}

func kubeVirtDataVolumeImage(ctx context.Context, namespace, serviceType string) string {
	cl, err := requireClient()
	if err != nil {
		return ""
	}
	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion("cdi.kubevirt.io/v1beta1")
	list.SetKind("DataVolumeList")
	if err := cl.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"paap.io/provision-mode": "kubevirt",
			"paap.io/service-type":   serviceType,
		},
	); err != nil || len(list.Items) == 0 {
		return ""
	}
	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].GetName() < list.Items[j].GetName() })
	image, _, _ := unstructured.NestedString(list.Items[0].Object, "spec", "source", "registry", "url")
	return image
}

func kubeVirtRuntimeResourceRequests(vm *unstructured.Unstructured) map[string]string {
	out := map[string]string{}
	if memory, _, _ := unstructured.NestedString(vm.Object, "spec", "template", "spec", "domain", "resources", "requests", "memory"); strings.TrimSpace(memory) != "" {
		out["memory"] = strings.TrimSpace(memory)
	}
	if cpu, _, _ := unstructured.NestedString(vm.Object, "spec", "template", "spec", "domain", "resources", "requests", "cpu"); strings.TrimSpace(cpu) != "" {
		out["cpu"] = strings.TrimSpace(cpu)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func kubeVirtRuntimePorts(ctx context.Context, namespace, serviceType string) []int32 {
	cl, err := requireClient()
	if err != nil {
		return nil
	}
	list := &corev1.ServiceList{}
	if err := cl.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"paap.io/provision-mode": "kubevirt",
			"paap.io/service-type":   serviceType,
		},
	); err != nil || len(list.Items) == 0 {
		return nil
	}
	ports := make([]int32, 0)
	seen := map[int32]struct{}{}
	for _, svc := range list.Items {
		for _, port := range svc.Spec.Ports {
			if port.Port <= 0 {
				continue
			}
			if _, ok := seen[port.Port]; ok {
				continue
			}
			seen[port.Port] = struct{}{}
			ports = append(ports, port.Port)
		}
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })
	return ports
}

func kubeVirtRuntimeSecrets(ctx context.Context, namespace, serviceType string) []RuntimeConfigObject {
	cl, err := requireClient()
	if err != nil {
		return nil
	}
	list := &corev1.SecretList{}
	if err := cl.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"paap.io/provision-mode": "kubevirt",
			"paap.io/service-type":   serviceType,
		},
	); err != nil || len(list.Items) == 0 {
		return nil
	}
	out := make([]RuntimeConfigObject, 0, len(list.Items))
	for _, secret := range list.Items {
		keys := make([]string, 0, len(secret.Data))
		for key := range secret.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out = append(out, RuntimeConfigObject{Name: secret.Name, Keys: keys})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func kubeVirtPrintableStatusPhase(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return "running"
	case "starting", "provisioning", "scheduling", "scheduled", "pending", "stopping", "paused":
		return "installing"
	case "errorunschedulable", "errpvcnotfound", "datavolumeerror", "crashloopbackoff", "failed":
		return "failed"
	default:
		return ""
	}
}

func kubeVirtReadyCondition(vm *unstructured.Unstructured) (bool, string) {
	conditions, _, _ := unstructured.NestedSlice(vm.Object, "status", "conditions")
	for _, raw := range conditions {
		condition, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		typ, _, _ := unstructured.NestedString(condition, "type")
		if !strings.EqualFold(typ, "Ready") {
			continue
		}
		status, _, _ := unstructured.NestedString(condition, "status")
		message, _, _ := unstructured.NestedString(condition, "message")
		reason, _, _ := unstructured.NestedString(condition, "reason")
		return strings.EqualFold(status, "True"), kvFirstNonEmpty(message, reason)
	}
	return false, ""
}

func deleteKubeVirtUnstructuredList(ctx context.Context, cl client.Client, apiVersion, kind, namespace string, labels client.MatchingLabels) error {
	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion(apiVersion)
	list.SetKind(kind)
	if err := cl.List(ctx, list, client.InNamespace(namespace), labels); err != nil {
		return fmt.Errorf("list kubevirt %s in %s: %w", kind, namespace, err)
	}
	for i := range list.Items {
		item := &list.Items[i]
		if err := cl.Delete(ctx, item); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete kubevirt %s %s/%s: %w", item.GetKind(), item.GetNamespace(), item.GetName(), err)
		}
	}
	return nil
}

func deleteKubeVirtServices(ctx context.Context, cl client.Client, namespace string, labels client.MatchingLabels) error {
	list := &corev1.ServiceList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), labels); err != nil {
		return fmt.Errorf("list kubevirt services in %s: %w", namespace, err)
	}
	for i := range list.Items {
		if err := cl.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete kubevirt service %s/%s: %w", list.Items[i].Namespace, list.Items[i].Name, err)
		}
	}
	return nil
}

func deleteKubeVirtSecrets(ctx context.Context, cl client.Client, namespace string, labels client.MatchingLabels) error {
	list := &corev1.SecretList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), labels); err != nil {
		return fmt.Errorf("list kubevirt secrets in %s: %w", namespace, err)
	}
	for i := range list.Items {
		if err := cl.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete kubevirt secret %s/%s: %w", list.Items[i].Namespace, list.Items[i].Name, err)
		}
	}
	return nil
}

func upsertKubeVirtNamespace(ctx context.Context, cl client.Client, name string, labels map[string]string) error {
	existing := &corev1.Namespace{}
	key := types.NamespacedName{Name: name}
	if err := cl.Get(ctx, key, existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get kubevirt namespace %s: %w", name, err)
		}
		return cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}})
	}
	existing.Labels = mergeKubeVirtStringMaps(existing.Labels, labels)
	return cl.Update(ctx, existing)
}

func upsertKubeVirtSecret(ctx context.Context, cl client.Client, desired *corev1.Secret) error {
	existing := &corev1.Secret{}
	key := types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}
	if err := cl.Get(ctx, key, existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get kubevirt secret %s/%s: %w", desired.Namespace, desired.Name, err)
		}
		return cl.Create(ctx, desired.DeepCopy())
	}
	existing.Labels = desired.Labels
	existing.Annotations = desired.Annotations
	existing.Type = desired.Type
	existing.Data = desired.Data
	return cl.Update(ctx, existing)
}

func upsertKubeVirtService(ctx context.Context, cl client.Client, desired *corev1.Service) error {
	existing := &corev1.Service{}
	key := types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}
	if err := cl.Get(ctx, key, existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get kubevirt service %s/%s: %w", desired.Namespace, desired.Name, err)
		}
		return cl.Create(ctx, desired.DeepCopy())
	}
	existing.Labels = desired.Labels
	existing.Annotations = desired.Annotations
	existing.Spec.Type = desired.Spec.Type
	existing.Spec.Ports = desired.Spec.Ports
	existing.Spec.Selector = desired.Spec.Selector
	return cl.Update(ctx, existing)
}

func upsertKubeVirtUnstructured(ctx context.Context, cl client.Client, desired *unstructured.Unstructured) error {
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(desired.GroupVersionKind())
	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	if err := cl.Get(ctx, key, existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get kubevirt %s %s/%s: %w", desired.GetKind(), desired.GetNamespace(), desired.GetName(), err)
		}
		return cl.Create(ctx, desired.DeepCopy())
	}
	existing.SetLabels(desired.GetLabels())
	existing.SetAnnotations(desired.GetAnnotations())
	if spec, ok := desired.Object["spec"]; ok {
		existing.Object["spec"] = spec
	}
	return cl.Update(ctx, existing)
}

func kubeVirtVirtualMachine(namespace, vmName string, spec kubeVirtRuntimeSpec, labels, annotations map[string]string) (*unstructured.Unstructured, *unstructured.Unstructured) {
	memory := kvFirstNonEmpty(spec.Memory, "1Gi")
	cpu := strings.TrimSpace(spec.CPU)
	diskName := "rootdisk"
	volumes := []interface{}{}
	disks := []interface{}{
		map[string]interface{}{
			"name": diskName,
			"disk": map[string]interface{}{"bus": "virtio"},
		},
	}

	var dataVolume *unstructured.Unstructured
	if strings.TrimSpace(spec.DiskSize) != "" {
		dataVolumeName := kubeVirtDNSLabel(kvFirstNonEmpty(spec.DataVolumeName, vmName+"-rootdisk"), vmName+"-rootdisk")
		volumes = append(volumes, map[string]interface{}{
			"name":       diskName,
			"dataVolume": map[string]interface{}{"name": dataVolumeName},
		})
		dataVolume = kubeVirtDataVolume(namespace, dataVolumeName, spec, labels, annotations)
	} else {
		volumes = append(volumes, map[string]interface{}{
			"name":          diskName,
			"containerDisk": map[string]interface{}{"image": spec.Image},
		})
	}

	if strings.TrimSpace(spec.CloudInitUserData) != "" {
		disks = append(disks, map[string]interface{}{
			"name": "cloudinitdisk",
			"disk": map[string]interface{}{"bus": "virtio"},
		})
		volumes = append(volumes, map[string]interface{}{
			"name": "cloudinitdisk",
			"cloudInitNoCloud": map[string]interface{}{
				"userData": spec.CloudInitUserData,
			},
		})
	}

	domain := map[string]interface{}{
		"devices": map[string]interface{}{
			"disks":      disks,
			"interfaces": []interface{}{map[string]interface{}{"name": "default", "bridge": map[string]interface{}{}}},
		},
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{
				"memory": memory,
			},
		},
	}
	if cpu != "" {
		requests := domain["resources"].(map[string]interface{})["requests"].(map[string]interface{})
		requests["cpu"] = cpu
		if cores, err := strconv.Atoi(cpu); err == nil && cores > 0 {
			domain["cpu"] = map[string]interface{}{"cores": int64(cores)}
		}
	}

	vmSpec := map[string]interface{}{
		"domain": domain,
		"networks": []interface{}{map[string]interface{}{
			"name": "default",
			"pod":  map[string]interface{}{},
		}},
		"volumes": volumes,
	}
	if probe := kubeVirtProbe(spec.Readiness); probe != nil {
		vmSpec["readinessProbe"] = probe
	}

	vm := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachine",
		"metadata": map[string]interface{}{
			"name":      vmName,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"runStrategy": "Always",
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": kubeVirtPodLabels(labels, vmName),
				},
				"spec": vmSpec,
			},
		},
	}}
	vm.SetLabels(labels)
	vm.SetAnnotations(annotations)
	return vm, dataVolume
}

func kubeVirtDataVolume(namespace, name string, spec kubeVirtRuntimeSpec, labels, annotations map[string]string) *unstructured.Unstructured {
	pvc := map[string]interface{}{
		"accessModes": []interface{}{"ReadWriteOnce"},
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{
				"storage": strings.TrimSpace(spec.DiskSize),
			},
		},
	}
	if strings.TrimSpace(spec.StorageClassName) != "" {
		pvc["storageClassName"] = strings.TrimSpace(spec.StorageClassName)
	}
	dataVolume := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cdi.kubevirt.io/v1beta1",
		"kind":       "DataVolume",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"registry": map[string]interface{}{"url": strings.TrimSpace(spec.Image)},
			},
			"pvc": pvc,
		},
	}}
	dataVolume.SetLabels(labels)
	dataVolume.SetAnnotations(annotations)
	return dataVolume
}

func kubeVirtService(namespace, vmName, serviceType string, ports []corev1.ServicePort, labels, annotations map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        vmName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: ports,
			Selector: map[string]string{
				"kubevirt.io/domain":     vmName,
				"paap.io/service-type":   serviceType,
				"paap.io/provision-mode": "kubevirt",
			},
		},
	}
}

func kubeVirtConnections(serviceType, namespace, serviceName string, ports []corev1.ServicePort, secretName string, credentials map[string]string, database string) []KubeVirtConnectionOutput {
	if len(ports) == 0 {
		return nil
	}
	host := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)
	conn := KubeVirtConnectionOutput{
		Name:       serviceName,
		Type:       serviceType,
		Host:       host,
		Port:       ports[0].Port,
		SecretName: secretName,
	}
	if _, ok := credentials["username"]; ok {
		conn.UsernameSecretKey = "username"
	}
	if _, ok := credentials["password"]; ok {
		conn.PasswordSecretKey = "password"
	}
	conn.URI = kubeVirtConnectionURI(serviceType, host, ports[0].Port, conn.UsernameSecretKey, conn.PasswordSecretKey, database)
	return []KubeVirtConnectionOutput{conn}
}

func kubeVirtConnectionURI(serviceType, host string, port int32, usernameKey, passwordKey, database string) string {
	user := "$(username)"
	if usernameKey == "" {
		user = ""
	}
	password := "$(password)"
	if passwordKey == "" {
		password = ""
	}
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "postgresql", "postgres":
		db := kvFirstNonEmpty(database, "postgres")
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", user, password, host, port, db)
	case "mysql":
		db := kvFirstNonEmpty(database, "mysql")
		return fmt.Sprintf("mysql://%s:%s@tcp(%s:%d)/%s", user, password, host, port, db)
	case "redis":
		if passwordKey == "" {
			return fmt.Sprintf("redis://%s:%d/0", host, port)
		}
		return fmt.Sprintf("redis://:%s@%s:%d/0", password, host, port)
	default:
		return fmt.Sprintf("%s:%d", host, port)
	}
}

func normalizeKubeVirtPorts(ports []kubeVirtRuntimeServicePort) ([]corev1.ServicePort, error) {
	if len(ports) == 0 {
		return nil, fmt.Errorf("kubevirt runtime spec ports are required")
	}
	result := make([]corev1.ServicePort, 0, len(ports))
	for i, port := range ports {
		if port.Port <= 0 || port.Port > 65535 {
			return nil, fmt.Errorf("kubevirt runtime spec ports[%d].port is invalid", i)
		}
		targetPort := port.TargetPort
		if targetPort == 0 {
			targetPort = port.Port
		}
		if targetPort <= 0 || targetPort > 65535 {
			return nil, fmt.Errorf("kubevirt runtime spec ports[%d].targetPort is invalid", i)
		}
		name := kubeVirtDNSLabel(kvFirstNonEmpty(port.Name, "tcp-"+strconv.Itoa(int(port.Port))), "tcp")
		protocol := corev1.ProtocolTCP
		if strings.EqualFold(strings.TrimSpace(port.Protocol), "udp") {
			protocol = corev1.ProtocolUDP
		}
		result = append(result, corev1.ServicePort{
			Name:       name,
			Protocol:   protocol,
			Port:       port.Port,
			TargetPort: intstr.FromInt32(targetPort),
		})
	}
	return result, nil
}

func validateKubeVirtProbe(probe *kubeVirtRuntimeProbe) error {
	if probe == nil {
		return nil
	}
	typ := strings.ToLower(strings.TrimSpace(probe.Type))
	if typ == "" {
		typ = "tcp"
	}
	switch typ {
	case "tcp", "http":
		if probe.Port <= 0 || probe.Port > 65535 {
			return fmt.Errorf("kubevirt runtime spec readiness.port is invalid")
		}
	case "exec":
		if len(probe.Command) == 0 {
			return fmt.Errorf("kubevirt runtime spec readiness.command is required")
		}
	default:
		return fmt.Errorf("kubevirt runtime spec readiness.type %q is invalid", probe.Type)
	}
	return nil
}

func validateKubeVirtMonitoring(monitoring *kubeVirtRuntimeMonitoring) error {
	if monitoring == nil || !monitoring.Enabled {
		return nil
	}
	if monitoring.Port <= 0 || monitoring.Port > 65535 {
		return fmt.Errorf("kubevirt runtime spec monitoring.port is invalid")
	}
	return nil
}

func kubeVirtProbe(probe *kubeVirtRuntimeProbe) map[string]interface{} {
	if probe == nil {
		return nil
	}
	out := map[string]interface{}{}
	switch strings.ToLower(strings.TrimSpace(probe.Type)) {
	case "http":
		path := strings.TrimSpace(probe.Path)
		if path == "" {
			path = "/"
		}
		out["httpGet"] = map[string]interface{}{
			"path": path,
			"port": int64(probe.Port),
		}
	case "exec":
		out["exec"] = map[string]interface{}{
			"command": probe.Command,
		}
	default:
		out["tcpSocket"] = map[string]interface{}{
			"port": int64(probe.Port),
		}
	}
	if probe.InitialDelaySeconds > 0 {
		out["initialDelaySeconds"] = int64(probe.InitialDelaySeconds)
	}
	if probe.PeriodSeconds > 0 {
		out["periodSeconds"] = int64(probe.PeriodSeconds)
	}
	if probe.TimeoutSeconds > 0 {
		out["timeoutSeconds"] = int64(probe.TimeoutSeconds)
	}
	if probe.FailureThreshold > 0 {
		out["failureThreshold"] = int64(probe.FailureThreshold)
	}
	return out
}

func kubeVirtRuntimeAnnotations(spec kubeVirtRuntimeSpec, annotations map[string]string) map[string]string {
	out := kvCopyStringMap(annotations)
	if out == nil {
		out = map[string]string{}
	}
	addJSON := func(key string, value interface{}) {
		if value == nil {
			return
		}
		data, err := json.Marshal(value)
		if err == nil && string(data) != "null" {
			out[key] = string(data)
		}
	}
	addJSON("paap.io/kubevirt-readiness", spec.Readiness)
	addJSON("paap.io/kubevirt-monitoring", spec.Monitoring)
	addJSON("paap.io/kubevirt-backup-policy", spec.BackupPolicy)
	return out
}

func kubeVirtMonitoringTarget(namespace, serviceType string, spec kubeVirtRuntimeSpec) string {
	if spec.Monitoring != nil && spec.Monitoring.Enabled {
		port := spec.Monitoring.Port
		path := strings.TrimSpace(spec.Monitoring.Path)
		if path == "" {
			path = "/metrics"
		}
		return fmt.Sprintf("namespace:%s;service=%s;port=%d;path=%s", namespace, serviceType, port, path)
	}
	return "namespace:" + namespace
}

func kubeVirtCredentials(serviceType string, provided map[string]string) (map[string]string, error) {
	credentials := map[string]string{}
	for key, value := range provided {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		credentials[key] = value
	}
	if _, ok := credentials["username"]; !ok && serviceType != "redis" {
		credentials["username"] = kubeVirtDefaultUsername(serviceType)
	}
	if _, ok := credentials["password"]; !ok {
		password, err := randomKubeVirtPassword()
		if err != nil {
			return nil, err
		}
		credentials["password"] = password
	}
	return credentials, nil
}

func kubeVirtDefaultUsername(serviceType string) string {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "postgresql", "postgres":
		return "postgres"
	case "mysql":
		return "root"
	default:
		return "user"
	}
}

func randomKubeVirtPassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate kubevirt credential: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func kubeVirtServiceLabels(input KubeVirtServiceResourceInput, vmName string) map[string]string {
	labels := map[string]string{
		"paap.io/app":            strings.TrimSpace(input.AppIdentifier),
		"paap.io/env":            strings.TrimSpace(input.EnvIdentifier),
		"paap.io/service":        strings.TrimSpace(input.ServiceType),
		"paap.io/service-type":   strings.TrimSpace(input.ServiceType),
		"paap.io/provision-mode": "kubevirt",
		"paap.io/managed-by":     "paap-server",
		"paap.io/vm":             vmName,
	}
	for key, value := range input.Labels {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		labels[key] = value
	}
	return labels
}

func kubeVirtPodLabels(labels map[string]string, vmName string) map[string]interface{} {
	result := map[string]interface{}{
		"kubevirt.io/domain": vmName,
	}
	for key, value := range labels {
		result[key] = value
	}
	return result
}

func kubeVirtDNSLabel(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = kubeVirtNameInvalidChars.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		value = strings.Trim(strings.ToLower(fallback), "-")
	}
	if len(value) > 63 {
		value = strings.Trim(value[:63], "-")
	}
	if value == "" {
		return "service"
	}
	return value
}

func kvCopyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func mergeKubeVirtStringMaps(base, overlay map[string]string) map[string]string {
	result := kvCopyStringMap(base)
	if result == nil {
		result = map[string]string{}
	}
	for key, value := range overlay {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		result[key] = value
	}
	return result
}

func kvFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
