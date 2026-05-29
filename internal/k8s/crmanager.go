package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	paapv1 "paap/api/v1"
)

var (
	scheme  = runtime.NewScheme()
	k8sClient client.Client
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
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

// CreateConfigMap creates a ConfigMap in the specified namespace
func CreateConfigMap(ctx context.Context, namespace, name string, data map[string]string, labels map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
	return k8sClient.Create(ctx, cm)
}

// CreateApplicationCR creates an Application CR in paap-system namespace
func CreateApplicationCR(ctx context.Context, name, identifier, description string) error {
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
	return k8sClient.Create(ctx, app)
}

// DeleteApplicationCR deletes an Application CR
func DeleteApplicationCR(ctx context.Context, identifier string) error {
	app := &paapv1.Application{}
	key := types.NamespacedName{Name: identifier, Namespace: "paap-system"}
	if err := k8sClient.Get(ctx, key, app); err != nil {
		return client.IgnoreNotFound(err)
	}
	return k8sClient.Delete(ctx, app)
}

// CreateEnvironmentCR creates an Environment CR in the app's CR namespace
func CreateEnvironmentCR(ctx context.Context, appIdentifier, envName, envIdentifier, primaryNS string, additionalNS []paapv1.AdditionalNamespace) error {
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
			Name:               envName,
			Identifier:         envIdentifier,
			PrimaryNamespace:   primaryNS,
			AdditionalNamespaces: additionalNS,
			Network: paapv1.NetworkSpec{
				Isolation: "NetworkPolicy",
			},
		},
	}
	return k8sClient.Create(ctx, env)
}

// DeleteEnvironmentCR deletes an Environment CR
func DeleteEnvironmentCR(ctx context.Context, appIdentifier, envIdentifier string) error {
	env := &paapv1.Environment{}
	key := types.NamespacedName{
		Name:      envIdentifier,
		Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
	}
	if err := k8sClient.Get(ctx, key, env); err != nil {
		return client.IgnoreNotFound(err)
	}
	return k8sClient.Delete(ctx, env)
}

// CreateServiceInstanceCR creates a ServiceInstance CR
func CreateServiceInstanceCR(ctx context.Context, appIdentifier, envIdentifier, svcType string, workloadRole paapv1.RoleSpec, manifestsRef *paapv1.ConfigMapReference) error {
	toolNS := fmt.Sprintf("%s-%s-%s", appIdentifier, envIdentifier, svcType)
	saName := fmt.Sprintf("%s-%s-%s", appIdentifier, envIdentifier, svcType)

	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", envIdentifier, svcType),
			Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
			Labels: map[string]string{
				"paap.io/app":  appIdentifier,
				"paap.io/env":  envIdentifier,
				"paap.io/tool": svcType,
			},
			Annotations: map[string]string{
				"paap.io/tool-namespace": toolNS,
				"paap.io/app":            appIdentifier,
				"paap.io/env":            envIdentifier,
				"paap.io/tool":           svcType,
			},
		},
		Spec: paapv1.ServiceInstanceSpec{
			EnvironmentRef: paapv1.ObjectReference{
				Name: envIdentifier,
			},
			Type:          svcType,
			ToolNamespace: toolNS,
			ServiceAccount: paapv1.ServiceAccountSpec{
				Name:      saName,
				Namespace: toolNS,
			},
			WorkloadRole:  workloadRole,
			ManifestsRef:  manifestsRef,
		},
	}
	return k8sClient.Create(ctx, svc)
}

// DeleteServiceInstanceCR deletes a ServiceInstance CR
func DeleteServiceInstanceCR(ctx context.Context, appIdentifier, envIdentifier, svcType string) error {
	svc := &paapv1.ServiceInstance{}
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", envIdentifier, svcType),
		Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
	}
	if err := k8sClient.Get(ctx, key, svc); err != nil {
		return client.IgnoreNotFound(err)
	}
	return k8sClient.Delete(ctx, svc)
}

// CreateComponentCR creates a Component CR
func CreateComponentCR(ctx context.Context, appIdentifier, envIdentifier, compName, compIdentifier, compType, image, tag string, replicas int32, targetNamespace string) error {
	comp := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", envIdentifier, compIdentifier),
			Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
			Labels: map[string]string{
				"paap.io/app": appIdentifier,
				"paap.io/env": envIdentifier,
			},
		},
		Spec: paapv1.ComponentSpec{
			EnvironmentRef: paapv1.ObjectReference{
				Name: envIdentifier,
			},
			Name:       compName,
			Identifier: compIdentifier,
			Type:       compType,
			ManagedBy:  "operator",
			Deployment: paapv1.DeploymentSpec{
				Namespace: targetNamespace,
				Image:     image,
				Tag:       tag,
				Replicas:  replicas,
			},
			Service: &paapv1.ServiceSpec{
				Port:       80,
				TargetPort: 8080,
				Type:       "ClusterIP",
			},
		},
	}
	return k8sClient.Create(ctx, comp)
}

// DeleteComponentCR deletes a Component CR
func DeleteComponentCR(ctx context.Context, appIdentifier, envIdentifier, compIdentifier string) error {
	comp := &paapv1.Component{}
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", envIdentifier, compIdentifier),
		Namespace: fmt.Sprintf("paap-app-%s", appIdentifier),
	}
	if err := k8sClient.Get(ctx, key, comp); err != nil {
		return client.IgnoreNotFound(err)
	}
	return k8sClient.Delete(ctx, comp)
}
