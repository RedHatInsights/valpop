package s3

import (
	"context"
	"io"

	impl "github.com/RedHatInsights/valpop/impl"
	minio "github.com/minio/minio-go/v7"
)

// S3Client interface abstracts the MinIO client for testing
type S3Client interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
}

// S3Service interface extends Implementation with S3-specific operations
type S3Service interface {
	impl.Implementation

	// S3-specific operations
	SetManifest(namespace, bucket string, timestamp int64, files impl.Manifest) error
	PopulateFn(addr, bucket, source, prefix, image string, timeout int64, minAssetRecords int64, cacheMaxAge int64) error
	CleanupCache(prefix, bucket string, timeout int64, minAssetRecords int64) error
}

// Note: Implementations of S3Service should also implement impl.Implementation
// The interface composition provides all the necessary methods for storage operations
