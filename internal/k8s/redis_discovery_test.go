package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDiscoverRedisConnectionPrefersMasterService(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-redis-headless", Namespace: "billing-dev-redis"},
			Spec: corev1.ServiceSpec{
				ClusterIP: corev1.ClusterIPNone,
				Ports:     []corev1.ServicePort{{Port: 6379}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "billing-dev-redis-replicas",
				Namespace: "billing-dev-redis",
				Labels:    map[string]string{"app.kubernetes.io/component": "replica"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 6379}}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "billing-dev-redis-master",
				Namespace: "billing-dev-redis",
				Labels:    map[string]string{"app.kubernetes.io/component": "master"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 6379}}},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-redis", Namespace: "billing-dev-redis"},
			Data:       map[string][]byte{"redis-password": []byte("secret")},
		},
	).Build())

	info, err := DiscoverRedisConnection(t.Context(), "billing-dev-redis")
	if err != nil {
		t.Fatalf("discover redis: %v", err)
	}
	if info.Host != "billing-dev-redis-master.billing-dev-redis.svc.cluster.local" {
		t.Fatalf("redis host = %q, want master service", info.Host)
	}
	if info.Password != "secret" {
		t.Fatalf("redis password = %q, want secret", info.Password)
	}
}
