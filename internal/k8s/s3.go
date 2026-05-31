package k8s

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Client 封装 S3/MinIO 操作
type S3Client struct {
	client     *minio.Client
	bucketName string
}

// NewS3Client 创建 S3 客户端
func NewS3Client(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*S3Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// 检查并创建 bucket
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}
	if !exists {
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &S3Client{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// UploadFile 上传文件到 S3
func (s *S3Client) UploadFile(ctx context.Context, objectName, filePath, contentType string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	_, err = s.client.PutObject(ctx, s.bucketName, objectName, file, fileInfo.Size(), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

// DownloadFile 从 S3 下载文件到本地
func (s *S3Client) DownloadFile(ctx context.Context, objectName, localPath string) error {
	// 确保目录存在
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	object, err := s.client.GetObject(ctx, s.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer object.Close()

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, object)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	return nil
}

// DeleteObject 删除 S3 对象
func (s *S3Client) DeleteObject(ctx context.Context, objectName string) error {
	return s.client.RemoveObject(ctx, s.bucketName, objectName, minio.RemoveObjectOptions{})
}

// ObjectExists 检查对象是否存在
func (s *S3Client) ObjectExists(ctx context.Context, objectName string) bool {
	_, err := s.client.StatObject(ctx, s.bucketName, objectName, minio.StatObjectOptions{})
	return err == nil
}

// ListObjects 列出指定前缀的对象
func (s *S3Client) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	var objects []string
	for object := range s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if object.Err != nil {
			return nil, object.Err
		}
		objects = append(objects, object.Key)
	}
	return objects, nil
}
