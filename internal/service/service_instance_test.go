package service

import (
	"context"
	"testing"

	"paap/internal/k8s"
	"paap/internal/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestListServiceInstancesEnrichesKubeVirtRuntimeState(t *testing.T) {
	previous := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previous) })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "billing-dev", Status: "running"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   "redis",
		ServiceName:   "redis",
		Status:        "installing",
		Namespace:     "billing-dev-redis",
		ProvisionMode: model.ServiceProvisionModeKubeVirt,
	}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create install: %v", err)
	}

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).Build())
	resources, err := k8s.BuildKubeVirtServiceResources(k8s.KubeVirtServiceResourceInput{
		AppIdentifier: app.Identifier,
		EnvIdentifier: env.Identifier,
		ServiceType:   inst.ServiceType,
		ServiceName:   inst.ServiceName,
		Namespace:     inst.Namespace,
		RuntimeSpec:   `{"image":"redis:7","cpu":"1","memory":"2Gi","ports":[{"port":6379}],"credentials":{"password":"secret"}}`,
	})
	if err != nil {
		t.Fatalf("build resources: %v", err)
	}
	resources.Service.Spec.ClusterIP = "10.96.0.77"
	if err := k8s.UpsertKubeVirtServiceResources(context.Background(), resources); err != nil {
		t.Fatalf("upsert resources: %v", err)
	}
	vm := &unstructured.Unstructured{}
	vm.SetAPIVersion("kubevirt.io/v1")
	vm.SetKind("VirtualMachine")
	if err := k8s.GetClient().Get(context.Background(), types.NamespacedName{Name: "redis", Namespace: "billing-dev-redis"}, vm); err != nil {
		t.Fatalf("get vm: %v", err)
	}
	vm.Object["status"] = map[string]interface{}{"printableStatus": "Running"}
	if err := k8s.GetClient().Update(context.Background(), vm); err != nil {
		t.Fatalf("update vm status: %v", err)
	}

	views, err := ListServiceInstances(context.Background(), db, env.ID)
	if err != nil {
		t.Fatalf("list service instances: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("views length = %d, want 1", len(views))
	}
	view := views[0]
	if view.Status != "running" {
		t.Fatalf("status = %q, want running", view.Status)
	}
	if view.ProvisionMode != model.ServiceProvisionModeKubeVirt {
		t.Fatalf("provision mode = %q", view.ProvisionMode)
	}
	if view.RuntimeServiceName != "redis" || view.ClusterIP != "10.96.0.77" {
		t.Fatalf("network info = name %q clusterIP %q", view.RuntimeServiceName, view.ClusterIP)
	}
	if view.RuntimeConfig == nil {
		t.Fatalf("runtime config is nil")
	}
	if view.RuntimeConfig.WorkloadKind != "VirtualMachine" || view.RuntimeConfig.Image != "redis:7" {
		t.Fatalf("runtime config = %#v", view.RuntimeConfig)
	}
	if len(view.RuntimeConfig.Ports) != 1 || view.RuntimeConfig.Ports[0] != 6379 {
		t.Fatalf("ports = %#v", view.RuntimeConfig.Ports)
	}
	if view.RuntimeConfig.Resources.Requests["memory"] != "2Gi" {
		t.Fatalf("resources = %#v", view.RuntimeConfig.Resources.Requests)
	}
}

func TestDiscoverServiceCredentialsReadsKubeVirtCredentialSecret(t *testing.T) {
	previous := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previous) })

	k8s.SetClient(fake.NewClientBuilder().WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-credentials",
			Namespace: "billing-dev-redis",
			Labels: map[string]string{
				"paap.io/provision-mode": "kubevirt",
				"paap.io/service-type":   "redis",
			},
		},
		Data: map[string][]byte{"password": []byte("secret")},
		Type: corev1.SecretTypeOpaque,
	}).Build())

	credentials, err := DiscoverServiceCredentials(context.Background(), "billing-dev-redis")
	if err != nil {
		t.Fatalf("discover credentials: %v", err)
	}
	if len(credentials) != 1 || credentials[0].Secret != "redis-credentials" || credentials[0].Kind != "password" || credentials[0].Value != "secret" {
		t.Fatalf("credentials = %#v", credentials)
	}
}
