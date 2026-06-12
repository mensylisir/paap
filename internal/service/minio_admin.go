package service

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"paap/internal/k8s"
)

type MinIOBucket struct {
	Name string
}

type MinIOObject struct {
	Key  string
	Size int64
}

func ListMinIOBuckets(ctx context.Context, info k8s.MinIOConnectionInfo) ([]MinIOBucket, error) {
	client, err := openMinIO(info)
	if err != nil {
		return nil, err
	}
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]MinIOBucket, 0, len(buckets))
	for _, bucket := range buckets {
		result = append(result, MinIOBucket{Name: bucket.Name})
	}
	return result, nil
}

func CreateMinIOBucket(ctx context.Context, info k8s.MinIOConnectionInfo, bucket string) error {
	if bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	client, err := openMinIO(info)
	if err != nil {
		return err
	}
	return client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
}

func DeleteMinIOBucket(ctx context.Context, info k8s.MinIOConnectionInfo, bucket string) error {
	if bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	client, err := openMinIO(info)
	if err != nil {
		return err
	}
	return client.RemoveBucket(ctx, bucket)
}

func ListMinIOObjects(ctx context.Context, info k8s.MinIOConnectionInfo, bucket, prefix string, limit int) ([]MinIOObject, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	client, err := openMinIO(info)
	if err != nil {
		return nil, err
	}
	objects := make([]MinIOObject, 0, limit)
	for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return nil, object.Err
		}
		objects = append(objects, MinIOObject{Key: object.Key, Size: object.Size})
		if len(objects) >= limit {
			break
		}
	}
	return objects, nil
}

func DeleteMinIOObject(ctx context.Context, info k8s.MinIOConnectionInfo, bucket, object string) error {
	if bucket == "" || object == "" {
		return fmt.Errorf("bucket and object are required")
	}
	client, err := openMinIO(info)
	if err != nil {
		return err
	}
	return client.RemoveObject(ctx, bucket, object, minio.RemoveObjectOptions{})
}

func openMinIO(info k8s.MinIOConnectionInfo) (*minio.Client, error) {
	return minio.New(info.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(info.AccessKey, info.SecretKey, ""),
		Secure: info.UseSSL,
	})
}
