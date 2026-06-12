package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDiscoverDatabaseConnectionFindsMySQLServiceAndSecret(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-mysql", Namespace: "billing-dev-mysql"},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 3306}}},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-mysql", Namespace: "billing-dev-mysql"},
			Data:       map[string][]byte{"mysql-root-password": []byte("secret")},
		},
	).Build())

	info, err := DiscoverDatabaseConnection(t.Context(), "billing-dev-mysql", "mysql")
	if err != nil {
		t.Fatalf("discover mysql: %v", err)
	}
	if info.Driver != "mysql" || info.Host != "billing-dev-mysql.billing-dev-mysql.svc.cluster.local" || info.Port != 3306 || info.Username != "root" || info.Password != "secret" {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestDiscoverDatabaseConnectionFindsPostgreSQLServiceAndSecret(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-postgresql", Namespace: "billing-dev-postgresql"},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 5432}}},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-postgresql", Namespace: "billing-dev-postgresql"},
			Data:       map[string][]byte{"postgres-password": []byte("secret")},
		},
	).Build())

	info, err := DiscoverDatabaseConnection(t.Context(), "billing-dev-postgresql", "postgresql")
	if err != nil {
		t.Fatalf("discover postgresql: %v", err)
	}
	if info.Driver != "pgx" || info.Host != "billing-dev-postgresql.billing-dev-postgresql.svc.cluster.local" || info.Port != 5432 || info.Username != "postgres" || info.Password != "secret" || info.Database != "postgres" {
		t.Fatalf("unexpected info: %#v", info)
	}
}
