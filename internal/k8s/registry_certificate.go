package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReadRegistryCACertificate returns the public CA certificate operators should
// install into node container runtimes. It never returns tls.key.
func ReadRegistryCACertificate(ctx context.Context, namespace, serviceType, releaseName string) ([]byte, string, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, "", err
	}
	return ReadRegistryCACertificateWithClient(ctx, cl, namespace, serviceType, releaseName)
}

func ReadRegistryCACertificateWithClient(ctx context.Context, cl client.Client, namespace, serviceType, releaseName string) ([]byte, string, error) {
	if cl == nil {
		return nil, "", fmt.Errorf("k8s client not initialized")
	}
	for _, secretName := range registryTLSSecretCandidates(serviceType, releaseName) {
		secret := &corev1.Secret{}
		if err := cl.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret); err != nil {
			continue
		}
		for _, key := range []string{"ca.crt", "tls.crt"} {
			if cert := secret.Data[key]; len(cert) > 0 {
				return cert, secretName + "/" + key, nil
			}
		}
	}

	secrets := &corev1.SecretList{}
	if err := cl.List(ctx, secrets, client.InNamespace(namespace)); err != nil {
		return nil, "", fmt.Errorf("list registry TLS secrets: %w", err)
	}
	for _, secret := range secrets.Items {
		if secret.Type != corev1.SecretTypeTLS && len(secret.Data["ca.crt"]) == 0 {
			continue
		}
		for _, key := range []string{"ca.crt", "tls.crt"} {
			if cert := secret.Data[key]; len(cert) > 0 {
				return cert, secret.Name + "/" + key, nil
			}
		}
	}
	return nil, "", fmt.Errorf("no registry CA certificate found in namespace %s", namespace)
}

func registryTLSSecretCandidates(serviceType, releaseName string) []string {
	if releaseName == "" {
		releaseName = serviceType
	}
	switch serviceType {
	case "harbor":
		return []string{
			releaseName + "-ingress",
			releaseName + "-harbor-ingress",
			releaseName + "-nginx",
			releaseName + "-harbor-nginx",
		}
	default:
		return []string{
			releaseName + "-tls",
			releaseName + "-registry-tls",
		}
	}
}
