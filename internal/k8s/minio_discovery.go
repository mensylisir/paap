package k8s

import (
	"context"
	"fmt"
)

type MinIOConnectionInfo struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

func DiscoverMinIOConnection(ctx context.Context, namespace string) (MinIOConnectionInfo, error) {
	cl, err := requireClient()
	if err != nil {
		return MinIOConnectionInfo{}, err
	}
	host, err := discoverServiceHost(ctx, cl, namespace, 9000)
	if err != nil {
		return MinIOConnectionInfo{}, err
	}
	accessKey, _ := discoverOptionalSecretValue(ctx, cl, namespace, []string{"root-user", "access-key", "accesskey"})
	secretKey, _ := discoverOptionalSecretValue(ctx, cl, namespace, []string{"root-password", "secret-key", "secretkey"})
	if accessKey == "" {
		accessKey = "minioadmin"
	}
	if secretKey == "" {
		secretKey = "minioadmin123"
	}
	return MinIOConnectionInfo{
		Endpoint:  fmt.Sprintf("%s:%d", host, 9000),
		AccessKey: accessKey,
		SecretKey: secretKey,
		UseSSL:    false,
	}, nil
}
