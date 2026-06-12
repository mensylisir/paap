package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDiscoverMiddlewareConnections(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "mongodb", Namespace: "tools"},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 27017, TargetPort: intstr.FromInt(27017)}}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "rabbitmq", Namespace: "tools"},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 15672, TargetPort: intstr.FromInt(15672)}}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "kafka", Namespace: "tools"},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 9092, TargetPort: intstr.FromInt(9092)}}},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "middleware-secret", Namespace: "tools"},
			Data: map[string][]byte{
				"mongodb-root-password": []byte("mongo-pass"),
				"rabbitmq-username":     []byte("ops"),
				"rabbitmq-password":     []byte("rabbit-pass"),
			},
		},
	).Build())

	mongo, err := DiscoverMongoDBConnection(t.Context(), "tools")
	if err != nil {
		t.Fatalf("mongodb discovery: %v", err)
	}
	if mongo.Host != "mongodb.tools.svc.cluster.local" || mongo.Password != "mongo-pass" {
		t.Fatalf("unexpected mongodb info %#v", mongo)
	}

	rabbit, err := DiscoverRabbitMQConnection(t.Context(), "tools")
	if err != nil {
		t.Fatalf("rabbitmq discovery: %v", err)
	}
	if rabbit.ManagementURL != "http://rabbitmq.tools.svc.cluster.local:15672" || rabbit.Username != "ops" || rabbit.Password != "rabbit-pass" {
		t.Fatalf("unexpected rabbitmq info %#v", rabbit)
	}

	kafka, err := DiscoverKafkaConnection(t.Context(), "tools")
	if err != nil {
		t.Fatalf("kafka discovery: %v", err)
	}
	if kafka.Broker != "kafka.tools.svc.cluster.local:9092" {
		t.Fatalf("unexpected kafka info %#v", kafka)
	}
}
