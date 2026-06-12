package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DatabaseConnectionInfo struct {
	Driver   string
	Host     string
	Port     int
	Username string
	Password string
	Database string
}

func DiscoverDatabaseConnection(ctx context.Context, namespace, serviceType string) (DatabaseConnectionInfo, error) {
	switch serviceType {
	case "mysql":
		return discoverSQLConnection(ctx, namespace, "mysql", 3306, "root", "", []string{"mysql-root-password", "mysql-password", "password"})
	case "postgresql":
		return discoverSQLConnection(ctx, namespace, "pgx", 5432, "postgres", "postgres", []string{"postgres-password", "password"})
	default:
		return DatabaseConnectionInfo{}, fmt.Errorf("database management is not supported for %s", serviceType)
	}
}

func discoverSQLConnection(ctx context.Context, namespace, driver string, port int, username, database string, passwordKeys []string) (DatabaseConnectionInfo, error) {
	cl, err := requireClient()
	if err != nil {
		return DatabaseConnectionInfo{}, err
	}
	host, err := discoverServiceHost(ctx, cl, namespace, port)
	if err != nil {
		return DatabaseConnectionInfo{}, err
	}
	password, err := discoverSecretValue(ctx, cl, namespace, passwordKeys)
	if err != nil {
		return DatabaseConnectionInfo{}, err
	}
	return DatabaseConnectionInfo{
		Driver:   driver,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Database: database,
	}, nil
}

func discoverServiceHost(ctx context.Context, cl client.Client, namespace string, port int) (string, error) {
	services := &corev1.ServiceList{}
	if err := cl.List(ctx, services, client.InNamespace(namespace)); err != nil {
		return "", fmt.Errorf("list services: %w", err)
	}
	for _, svc := range services.Items {
		for _, svcPort := range svc.Spec.Ports {
			if int(svcPort.Port) == port {
				return fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, namespace), nil
			}
		}
	}
	return "", fmt.Errorf("no service exposes port %d in namespace %s", port, namespace)
}

func discoverSecretValue(ctx context.Context, cl client.Client, namespace string, keys []string) (string, error) {
	secrets := &corev1.SecretList{}
	if err := cl.List(ctx, secrets, client.InNamespace(namespace)); err != nil {
		return "", fmt.Errorf("list secrets: %w", err)
	}
	for _, secret := range secrets.Items {
		for _, key := range keys {
			if value, ok := secret.Data[key]; ok && len(value) > 0 {
				return string(value), nil
			}
		}
	}
	return "", fmt.Errorf("no database password secret found in namespace %s", namespace)
}
