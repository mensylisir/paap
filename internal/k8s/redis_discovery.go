package k8s

import (
	"context"
	"fmt"
	"strings"

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
	host, err := discoverRedisWritableHost(ctx, cl, namespace)
	if err != nil {
		return RedisConnectionInfo{}, err
	}
	password, _ := discoverOptionalSecretValue(ctx, cl, namespace, []string{"redis-password", "password"})
	return RedisConnectionInfo{Host: host, Port: 6379, Password: password}, nil
}

func discoverRedisWritableHost(ctx context.Context, cl client.Client, namespace string) (string, error) {
	services := &corev1.ServiceList{}
	if err := cl.List(ctx, services, client.InNamespace(namespace)); err != nil {
		return "", fmt.Errorf("list redis services: %w", err)
	}
	var fallback *corev1.Service
	for i := range services.Items {
		svc := &services.Items[i]
		if !redisServiceExposesPort(svc, 6379) {
			continue
		}
		if redisServiceLooksLikeMaster(svc) {
			return serviceHost(svc, namespace), nil
		}
		if fallback == nil && redisServiceLooksWritableFallback(svc) {
			fallback = svc
		}
	}
	if fallback != nil {
		return serviceHost(fallback, namespace), nil
	}
	for i := range services.Items {
		svc := &services.Items[i]
		if redisServiceExposesPort(svc, 6379) {
			return serviceHost(svc, namespace), nil
		}
	}
	return "", fmt.Errorf("no redis service exposes port 6379 in namespace %s", namespace)
}

func redisServiceExposesPort(svc *corev1.Service, port int) bool {
	for _, svcPort := range svc.Spec.Ports {
		if int(svcPort.Port) == port {
			return true
		}
	}
	return false
}

func redisServiceLooksLikeMaster(svc *corev1.Service) bool {
	component := strings.ToLower(svc.Labels["app.kubernetes.io/component"])
	name := strings.ToLower(svc.Name)
	return component == "master" || strings.Contains(name, "master")
}

func redisServiceLooksWritableFallback(svc *corev1.Service) bool {
	name := strings.ToLower(svc.Name)
	if svc.Spec.ClusterIP == corev1.ClusterIPNone {
		return false
	}
	if strings.Contains(name, "replica") || strings.Contains(name, "slave") || strings.Contains(name, "sentinel") || strings.Contains(name, "headless") {
		return false
	}
	return true
}

func serviceHost(svc *corev1.Service, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, namespace)
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
