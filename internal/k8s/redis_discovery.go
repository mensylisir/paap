package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RedisConnectionInfo struct {
	Host     string
	Port     int
	Password string
}

func DiscoverRedisConnection(ctx context.Context, namespace string) (RedisConnectionInfo, error) {
	cl, err := requireClient()
	if err != nil {
		return RedisConnectionInfo{}, err
	}
	host, err := discoverServiceHost(ctx, cl, namespace, 6379)
	if err != nil {
		return RedisConnectionInfo{}, err
	}
	password, _ := discoverOptionalSecretValue(ctx, cl, namespace, []string{"redis-password", "password"})
	return RedisConnectionInfo{Host: host, Port: 6379, Password: password}, nil
}

func discoverOptionalSecretValue(ctx context.Context, cl client.Client, namespace string, keys []string) (string, bool) {
	secrets := &corev1.SecretList{}
	if err := cl.List(ctx, secrets, client.InNamespace(namespace)); err != nil {
		return "", false
	}
	for _, secret := range secrets.Items {
		for _, key := range keys {
			if value, ok := secret.Data[key]; ok && len(value) > 0 {
				return string(value), true
			}
		}
	}
	return "", false
}

func RedisAddress(info RedisConnectionInfo) string {
	return fmt.Sprintf("%s:%d", info.Host, info.Port)
}
