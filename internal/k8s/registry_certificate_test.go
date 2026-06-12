package k8s

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReadRegistryCACertificatePrefersCAAndDoesNotExposePrivateKey(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-registry-tls", Namespace: "billing-dev-registry"},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"ca.crt":  []byte("-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"),
				"tls.crt": []byte("-----BEGIN CERTIFICATE-----\nleaf\n-----END CERTIFICATE-----\n"),
				"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----\n"),
			},
		},
	).Build())

	cert, source, err := ReadRegistryCACertificate(t.Context(), "billing-dev-registry", "registry", "billing-dev-registry")
	if err != nil {
		t.Fatalf("read ca: %v", err)
	}
	if source != "billing-dev-registry-tls/ca.crt" {
		t.Fatalf("source = %q", source)
	}
	if !strings.Contains(string(cert), "ca") || strings.Contains(string(cert), "PRIVATE KEY") {
		t.Fatalf("unexpected cert payload: %s", string(cert))
	}
}

func TestReadRegistryCACertificateFindsHarborIngressSecret(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-prod-harbor-ingress", Namespace: "billing-prod-harbor"},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{"ca.crt": []byte("harbor-ca")},
		},
	).Build())

	cert, source, err := ReadRegistryCACertificate(t.Context(), "billing-prod-harbor", "harbor", "billing-prod-harbor")
	if err != nil {
		t.Fatalf("read harbor ca: %v", err)
	}
	if string(cert) != "harbor-ca" || source != "billing-prod-harbor-ingress/ca.crt" {
		t.Fatalf("unexpected cert/source: %q %q", string(cert), source)
	}
}
