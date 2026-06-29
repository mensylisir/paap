package k8s

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildKubeVirtServiceResourcesBuildsPostgreSQLVM(t *testing.T) {
	resources, err := BuildKubeVirtServiceResources(KubeVirtServiceResourceInput{
		AppIdentifier: "billing",
		EnvIdentifier: "dev",
		ServiceType:   "postgresql",
		ServiceName:   "billing-dev-postgresql",
		Namespace:     "billing-dev-postgresql",
		RuntimeSpec: `{
			"image": "docker.io/library/postgres:16",
			"cpu": "2",
			"memory": "4Gi",
			"diskSize": "20Gi",
			"storageClassName": "standard",
			"cloudInitUserData": "#cloud-config\nruncmd:\n  - systemctl enable postgresql",
			"database": "appdb",
			"ports": [{"name": "postgresql", "port": 5432}],
			"credentials": {"username": "app", "password": "secret"},
			"readiness": {"type": "tcp", "port": 5432, "initialDelaySeconds": 15},
			"monitoring": {"enabled": true, "port": 9187, "path": "/metrics"},
			"backupPolicy": {"enabled": true, "schedule": "0 2 * * *", "retention": "7d"}
		}`,
		Labels: map[string]string{"paap.io/category": "database"},
	})
	if err != nil {
		t.Fatalf("BuildKubeVirtServiceResources() error = %v", err)
	}
	if resources.VirtualMachine == nil || resources.VirtualMachine.GetKind() != "VirtualMachine" {
		t.Fatalf("expected VirtualMachine, got %#v", resources.VirtualMachine)
	}
	if resources.VirtualMachine.GetAPIVersion() != "kubevirt.io/v1" {
		t.Fatalf("vm apiVersion = %q", resources.VirtualMachine.GetAPIVersion())
	}
	if resources.VirtualMachine.GetName() != "billing-dev-postgresql" || resources.VirtualMachine.GetNamespace() != "billing-dev-postgresql" {
		t.Fatalf("unexpected vm identity %s/%s", resources.VirtualMachine.GetNamespace(), resources.VirtualMachine.GetName())
	}
	if resources.VirtualMachine.GetLabels()["paap.io/service-type"] != "postgresql" {
		t.Fatalf("vm labels = %#v", resources.VirtualMachine.GetLabels())
	}
	runStrategy, _, _ := unstructured.NestedString(resources.VirtualMachine.Object, "spec", "runStrategy")
	if runStrategy != "Always" {
		t.Fatalf("runStrategy = %q", runStrategy)
	}
	memory, _, _ := unstructured.NestedString(resources.VirtualMachine.Object, "spec", "template", "spec", "domain", "resources", "requests", "memory")
	if memory != "4Gi" {
		t.Fatalf("memory request = %q", memory)
	}
	cores, _, _ := unstructured.NestedInt64(resources.VirtualMachine.Object, "spec", "template", "spec", "domain", "cpu", "cores")
	if cores != 2 {
		t.Fatalf("cpu cores = %d", cores)
	}
	volumes, _, _ := unstructured.NestedSlice(resources.VirtualMachine.Object, "spec", "template", "spec", "volumes")
	if len(volumes) != 2 {
		t.Fatalf("vm volumes = %#v", volumes)
	}
	cloudInitVolume, ok := volumes[1].(map[string]interface{})
	if !ok {
		t.Fatalf("cloud-init volume = %#v", volumes[1])
	}
	cloudInit, ok := cloudInitVolume["cloudInitNoCloud"].(map[string]interface{})
	if !ok || !strings.Contains(kubeVirtTestString(cloudInit["userData"]), "systemctl enable postgresql") {
		t.Fatalf("cloud-init not wired: %#v", cloudInitVolume)
	}

	if resources.DataVolume == nil || resources.DataVolume.GetKind() != "DataVolume" {
		t.Fatalf("expected DataVolume, got %#v", resources.DataVolume)
	}
	image, _, _ := unstructured.NestedString(resources.DataVolume.Object, "spec", "source", "registry", "url")
	if image != "docker.io/library/postgres:16" {
		t.Fatalf("data volume source image = %q", image)
	}
	storage, _, _ := unstructured.NestedString(resources.DataVolume.Object, "spec", "pvc", "resources", "requests", "storage")
	if storage != "20Gi" {
		t.Fatalf("data volume storage = %q", storage)
	}

	if resources.Secret == nil || string(resources.Secret.Data["username"]) != "app" || string(resources.Secret.Data["password"]) != "secret" {
		t.Fatalf("unexpected secret data: %#v", resources.Secret)
	}
	if resources.Service == nil || resources.Service.Name != "billing-dev-postgresql" {
		t.Fatalf("unexpected service: %#v", resources.Service)
	}
	if resources.Service.Spec.Selector["kubevirt.io/domain"] != "billing-dev-postgresql" {
		t.Fatalf("service selector = %#v", resources.Service.Spec.Selector)
	}
	if len(resources.Service.Spec.Ports) != 1 || resources.Service.Spec.Ports[0].Port != 5432 || resources.Service.Spec.Ports[0].TargetPort.IntVal != 5432 {
		t.Fatalf("service ports = %#v", resources.Service.Spec.Ports)
	}
	if !strings.Contains(resources.MonitoringTarget, "port=9187") || !strings.Contains(resources.MonitoringTarget, "path=/metrics") {
		t.Fatalf("monitoring target = %q", resources.MonitoringTarget)
	}
	probe, _, _ := unstructured.NestedMap(resources.VirtualMachine.Object, "spec", "template", "spec", "readinessProbe")
	if probe["tcpSocket"] == nil || probe["initialDelaySeconds"] != int64(15) {
		t.Fatalf("readiness probe not wired: %#v", probe)
	}
	annotations := resources.VirtualMachine.GetAnnotations()
	if !strings.Contains(annotations["paap.io/kubevirt-monitoring"], `"port":9187`) {
		t.Fatalf("monitoring annotation = %#v", annotations)
	}
	if !strings.Contains(annotations["paap.io/kubevirt-backup-policy"], `"retention":"7d"`) {
		t.Fatalf("backup annotation = %#v", annotations)
	}
	if len(resources.Connections) != 1 {
		t.Fatalf("connections = %#v", resources.Connections)
	}
	conn := resources.Connections[0]
	if conn.Host != "billing-dev-postgresql.billing-dev-postgresql.svc.cluster.local" || conn.Port != 5432 {
		t.Fatalf("connection endpoint = %#v", conn)
	}
	if conn.UsernameSecretKey != "username" || conn.PasswordSecretKey != "password" || conn.SecretName != "billing-dev-postgresql-credentials" {
		t.Fatalf("connection secret refs = %#v", conn)
	}
	if !strings.Contains(conn.URI, "/appdb") {
		t.Fatalf("connection uri = %q", conn.URI)
	}
}

func kubeVirtTestString(value interface{}) string {
	return fmt.Sprint(value)
}

func TestBuildKubeVirtServiceResourcesUsesContainerDiskWithoutDataVolume(t *testing.T) {
	resources, err := BuildKubeVirtServiceResources(KubeVirtServiceResourceInput{
		AppIdentifier: "cache",
		EnvIdentifier: "test",
		ServiceType:   "redis",
		ServiceName:   "redis-vm",
		Namespace:     "cache-test-redis",
		RuntimeSpec:   `{"image":"docker.io/library/redis:7","ports":[{"port":6379}]}`,
	})
	if err != nil {
		t.Fatalf("BuildKubeVirtServiceResources() error = %v", err)
	}
	if resources.DataVolume != nil {
		t.Fatalf("expected no DataVolume without diskSize, got %#v", resources.DataVolume)
	}
	volumes, _, _ := unstructured.NestedSlice(resources.VirtualMachine.Object, "spec", "template", "spec", "volumes")
	if len(volumes) != 1 {
		t.Fatalf("vm volumes = %#v", volumes)
	}
	rootVolume, ok := volumes[0].(map[string]interface{})
	if !ok {
		t.Fatalf("root volume = %#v", volumes[0])
	}
	containerDisk, ok := rootVolume["containerDisk"].(map[string]interface{})
	if !ok {
		t.Fatalf("container disk not wired: %#v", rootVolume)
	}
	image := kubeVirtTestString(containerDisk["image"])
	if image != "docker.io/library/redis:7" {
		t.Fatalf("container disk image = %q", image)
	}
	if _, ok := resources.Secret.Data["username"]; ok {
		t.Fatalf("redis secret should not invent username: %#v", resources.Secret.Data)
	}
	if string(resources.Secret.Data["password"]) == "" {
		t.Fatalf("redis password should be generated")
	}
}

func TestBuildKubeVirtServiceResourcesRejectsInvalidRuntimeSpec(t *testing.T) {
	base := KubeVirtServiceResourceInput{
		AppIdentifier: "billing",
		EnvIdentifier: "dev",
		ServiceType:   "postgresql",
		ServiceName:   "postgresql",
		Namespace:     "billing-dev-postgresql",
	}
	cases := []struct {
		name        string
		runtimeSpec string
		want        string
	}{
		{name: "missing image", runtimeSpec: `{"ports":[{"port":5432}]}`, want: "image is required"},
		{name: "missing ports", runtimeSpec: `{"image":"postgres:16"}`, want: "ports are required"},
		{name: "invalid port", runtimeSpec: `{"image":"postgres:16","ports":[{"port":70000}]}`, want: "ports[0].port is invalid"},
		{name: "invalid readiness", runtimeSpec: `{"image":"postgres:16","ports":[{"port":5432}],"readiness":{"type":"http","port":70000}}`, want: "readiness.port is invalid"},
		{name: "invalid monitoring", runtimeSpec: `{"image":"postgres:16","ports":[{"port":5432}],"monitoring":{"enabled":true,"port":70000}}`, want: "monitoring.port is invalid"},
		{name: "bad json", runtimeSpec: `{`, want: "parse kubevirt runtime spec"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			input.RuntimeSpec = tc.runtimeSpec
			_, err := BuildKubeVirtServiceResources(input)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want contains %q", err, tc.want)
			}
		})
	}
}

func TestUpsertKubeVirtServiceResourcesCreatesRuntimeObjects(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	SetClient(fake.NewClientBuilder().WithScheme(testScheme).Build())

	resources, err := BuildKubeVirtServiceResources(KubeVirtServiceResourceInput{
		AppIdentifier: "billing",
		EnvIdentifier: "dev",
		ServiceType:   "postgresql",
		ServiceName:   "billing-dev-postgresql",
		Namespace:     "billing-dev-postgresql",
		RuntimeSpec:   `{"image":"postgres:16","diskSize":"10Gi","ports":[{"port":5432}],"credentials":{"username":"app","password":"secret"}}`,
	})
	if err != nil {
		t.Fatalf("build resources: %v", err)
	}

	if err := UpsertKubeVirtServiceResources(context.Background(), resources); err != nil {
		t.Fatalf("upsert resources: %v", err)
	}

	var namespace corev1.Namespace
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "billing-dev-postgresql"}, &namespace); err != nil {
		t.Fatalf("namespace not created: %v", err)
	}
	var secret corev1.Secret
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "billing-dev-postgresql-credentials", Namespace: "billing-dev-postgresql"}, &secret); err != nil {
		t.Fatalf("secret not created: %v", err)
	}
	if string(secret.Data["password"]) != "secret" {
		t.Fatalf("secret password = %q", string(secret.Data["password"]))
	}
	var service corev1.Service
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "billing-dev-postgresql", Namespace: "billing-dev-postgresql"}, &service); err != nil {
		t.Fatalf("service not created: %v", err)
	}
	if service.Spec.Selector["paap.io/provision-mode"] != "kubevirt" {
		t.Fatalf("service selector = %#v", service.Spec.Selector)
	}
	dataVolume := &unstructured.Unstructured{}
	dataVolume.SetAPIVersion("cdi.kubevirt.io/v1beta1")
	dataVolume.SetKind("DataVolume")
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "billing-dev-postgresql-rootdisk", Namespace: "billing-dev-postgresql"}, dataVolume); err != nil {
		t.Fatalf("data volume not created: %v", err)
	}
	vm := &unstructured.Unstructured{}
	vm.SetAPIVersion("kubevirt.io/v1")
	vm.SetKind("VirtualMachine")
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "billing-dev-postgresql", Namespace: "billing-dev-postgresql"}, vm); err != nil {
		t.Fatalf("virtual machine not created: %v", err)
	}
	runStrategy, _, _ := unstructured.NestedString(vm.Object, "spec", "runStrategy")
	if runStrategy != "Always" {
		t.Fatalf("vm runStrategy = %q", runStrategy)
	}
}

func TestDeleteKubeVirtServiceResourcesDeletesRuntimeObjects(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	SetClient(fake.NewClientBuilder().WithScheme(testScheme).Build())

	resources, err := BuildKubeVirtServiceResources(KubeVirtServiceResourceInput{
		AppIdentifier: "billing",
		EnvIdentifier: "dev",
		ServiceType:   "redis",
		ServiceName:   "redis",
		Namespace:     "billing-dev-redis",
		RuntimeSpec:   `{"image":"redis:7","diskSize":"5Gi","ports":[{"port":6379}],"credentials":{"password":"secret"}}`,
	})
	if err != nil {
		t.Fatalf("build resources: %v", err)
	}
	if err := UpsertKubeVirtServiceResources(context.Background(), resources); err != nil {
		t.Fatalf("upsert resources: %v", err)
	}

	if err := DeleteKubeVirtServiceResources(context.Background(), "billing-dev-redis", "redis"); err != nil {
		t.Fatalf("delete resources: %v", err)
	}

	var namespace corev1.Namespace
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "billing-dev-redis"}, &namespace); err == nil {
		t.Fatalf("namespace should be deleted")
	}
	var service corev1.Service
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "redis", Namespace: "billing-dev-redis"}, &service); err == nil {
		t.Fatalf("service should be deleted")
	}
	vm := &unstructured.Unstructured{}
	vm.SetAPIVersion("kubevirt.io/v1")
	vm.SetKind("VirtualMachine")
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "redis", Namespace: "billing-dev-redis"}, vm); err == nil {
		t.Fatalf("virtual machine should be deleted")
	}
}

func TestDiscoverKubeVirtServiceRuntimeConfig(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	SetClient(fake.NewClientBuilder().WithScheme(testScheme).Build())

	resources, err := BuildKubeVirtServiceResources(KubeVirtServiceResourceInput{
		AppIdentifier: "billing",
		EnvIdentifier: "dev",
		ServiceType:   "postgresql",
		ServiceName:   "postgresql",
		Namespace:     "billing-dev-postgresql",
		RuntimeSpec:   `{"image":"postgres:16","cpu":"2","memory":"4Gi","diskSize":"10Gi","ports":[{"port":5432}],"credentials":{"username":"app","password":"secret"}}`,
	})
	if err != nil {
		t.Fatalf("build resources: %v", err)
	}
	resources.Service.Spec.ClusterIP = "10.96.0.42"
	if err := UpsertKubeVirtServiceResources(context.Background(), resources); err != nil {
		t.Fatalf("upsert resources: %v", err)
	}

	cfg, err := DiscoverKubeVirtServiceRuntimeConfig(context.Background(), "billing-dev-postgresql", "postgresql")
	if err != nil {
		t.Fatalf("discover kubevirt runtime config: %v", err)
	}
	if cfg == nil {
		t.Fatalf("runtime config is nil")
	}
	if cfg.WorkloadKind != "VirtualMachine" || cfg.WorkloadName != "postgresql" || cfg.Image != "postgres:16" {
		t.Fatalf("unexpected runtime identity: %#v", cfg)
	}
	if cfg.ServiceName != "postgresql" {
		t.Fatalf("service name = %q", cfg.ServiceName)
	}
	if len(cfg.Ports) != 1 || cfg.Ports[0] != 5432 {
		t.Fatalf("ports = %#v", cfg.Ports)
	}
	if cfg.Resources.Requests["cpu"] != "2" || cfg.Resources.Requests["memory"] != "4Gi" {
		t.Fatalf("resource requests = %#v", cfg.Resources.Requests)
	}
	if len(cfg.Secrets) != 1 || cfg.Secrets[0].Name != "postgresql-credentials" || len(cfg.Secrets[0].Keys) != 2 {
		t.Fatalf("secrets = %#v", cfg.Secrets)
	}
}

func TestDiscoverKubeVirtServiceStatusUsesPrintableStatus(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	vm := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachine",
		"metadata": map[string]interface{}{
			"name":      "redis",
			"namespace": "billing-dev-redis",
			"labels": map[string]interface{}{
				"paap.io/provision-mode": "kubevirt",
				"paap.io/service-type":   "redis",
			},
		},
		"status": map[string]interface{}{
			"printableStatus": "Running",
		},
	}}
	SetClient(fake.NewClientBuilder().WithObjects(vm).Build())

	status, err := DiscoverKubeVirtServiceStatus(context.Background(), "billing-dev-redis", "redis")
	if err != nil {
		t.Fatalf("discover kubevirt status: %v", err)
	}
	if status == nil || status.Phase != "running" || status.Message != "Running" {
		t.Fatalf("status = %#v", status)
	}
}
