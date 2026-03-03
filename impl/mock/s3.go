package mock

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	impl "github.com/RedHatInsights/valpop/impl"
	minio "github.com/minio/minio-go/v7"

	"github.com/RedHatInsights/valpop/impl/s3"
)

// S3Client implements the s3.S3Client interface for testing
type S3Client struct {
	Objects    map[string][]byte // bucketName/objectName -> content
	ObjectInfo map[string]minio.ObjectInfo
	Errors     map[string]error // operation -> error to return
}

func NewS3Client() *S3Client {
	return &S3Client{
		Objects:    make(map[string][]byte),
		ObjectInfo: make(map[string]minio.ObjectInfo),
		Errors:     make(map[string]error),
	}
}

func (m *S3Client) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	key := bucketName + "/" + objectName
	if err, exists := m.Errors["PutObject"]; exists {
		return minio.UploadInfo{}, err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return minio.UploadInfo{}, err
	}

	m.Objects[key] = data
	m.ObjectInfo[key] = minio.ObjectInfo{
		Key:  objectName,
		Size: int64(len(data)),
	}

	return minio.UploadInfo{
		Size: int64(len(data)),
		Key:  objectName,
	}, nil
}

func (m *S3Client) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	key := bucketName + "/" + objectName
	if err, exists := m.Errors["GetObject"]; exists {
		return nil, err
	}

	_, exists := m.Objects[key]
	if !exists {
		return nil, fmt.Errorf("object not found")
	}

	// Create a mock object that implements the required interface
	return &minio.Object{}, nil // Note: This is simplified for testing
}

func (m *S3Client) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	key := bucketName + "/" + objectName
	if err, exists := m.Errors["RemoveObject"]; exists {
		return err
	}

	delete(m.Objects, key)
	delete(m.ObjectInfo, key)
	return nil
}

func (m *S3Client) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo)

	go func() {
		defer close(ch)

		if err, exists := m.Errors["ListObjects"]; exists {
			ch <- minio.ObjectInfo{Err: err}
			return
		}

		for key, info := range m.ObjectInfo {
			if strings.HasPrefix(key, bucketName+"/") {
				objectName := strings.TrimPrefix(key, bucketName+"/")
				if strings.HasPrefix(objectName, opts.Prefix) {
					info.Key = objectName
					ch <- info
				}
			}
		}
	}()

	return ch
}

// S3Service implements the s3.S3Service interface for testing
type S3Service struct {
	StoredItems     map[string]string      // key -> content
	StoredManifests map[string]s3.Manifest // key -> manifest
	Operations      []string               // Track operations called
	Errors          map[string]error       // operation -> error to return
}

func NewS3Service() *S3Service {
	return &S3Service{
		StoredItems:     make(map[string]string),
		StoredManifests: make(map[string]s3.Manifest),
		Operations:      []string{},
		Errors:          make(map[string]error),
	}
}

func (m *S3Service) SetItem(namespace, filepath, contentType, bucket string, timestamp int64, contents string) error {
	m.Operations = append(m.Operations, "SetItem")
	if err, exists := m.Errors["SetItem"]; exists && err != nil {
		return err
	}

	key := impl.MakeDataKey(namespace, filepath)
	m.StoredItems[key] = contents
	return nil
}

func (m *S3Service) SetManifest(namespace, bucket string, timestamp int64, files s3.Manifest) error {
	m.Operations = append(m.Operations, "SetManifest")
	if err, exists := m.Errors["SetManifest"]; exists {
		return err
	}

	key := impl.MakeManifestKey(namespace, timestamp)
	m.StoredManifests[key] = files
	return nil
}

func (m *S3Service) PopulateFn(addr, bucket, source, prefix string, timeout int64, minAssetRecords int64) error {
	m.Operations = append(m.Operations, "PopulateFn")
	if err, exists := m.Errors["PopulateFn"]; exists {
		return err
	}

	// For mock service, only track the operation without actual filesystem access
	// Real filesystem operations should be tested in integration tests
	return nil
}

func (m *S3Service) CleanupCache(prefix, bucket string, timeout int64, minAssetRecords int64) error {
	m.Operations = append(m.Operations, "CleanupCache")
	if err, exists := m.Errors["CleanupCache"]; exists {
		return err
	}

	// Collect manifests for this prefix using the same logic as production
	currentTime := time.Now().Unix()
	allManifests := []impl.ManifestInfo{}

	for key, files := range m.StoredManifests {
		if strings.HasPrefix(key, fmt.Sprintf("manifests/%s/", prefix)) {
			timestampStr := strings.TrimPrefix(key, fmt.Sprintf("manifests/%s/", prefix))
			if timestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
				allManifests = append(allManifests, impl.ManifestInfo{
					Key:       key,
					Timestamp: timestamp,
					Files:     files,
				})
			}
		}
	}

	// Use common logic to determine what to delete
	toDelete, toKeep := impl.SeparateManifests(allManifests, currentTime, timeout, minAssetRecords)

	// Determine which files to delete
	filesToDelete := impl.DetermineFilesToDelete(toDelete, toKeep, []string{"fedmods.json"})

	// Delete old files from storage
	for _, file := range filesToDelete {
		key := impl.MakeDataKey(prefix, file)
		delete(m.StoredItems, key)
	}

	// Delete old manifests
	for _, manifest := range toDelete {
		delete(m.StoredManifests, manifest.Key)
	}

	return nil
}

func (m *S3Service) StartPopulate(namespace, bucket string, timestamp int64) error {
	m.Operations = append(m.Operations, "StartPopulate")
	return nil
}

func (m *S3Service) EndPopulate(namespace, bucket string, timestamp int64) error {
	m.Operations = append(m.Operations, "EndPopulate")
	return nil
}

func (m *S3Service) Close() {
	m.Operations = append(m.Operations, "Close")
}

// Helper methods for testing
func (m *S3Service) GetStoredItem(namespace, filepath string) (string, bool) {
	key := impl.MakeDataKey(namespace, filepath)
	content, exists := m.StoredItems[key]
	return content, exists
}

func (m *S3Service) GetStoredManifest(namespace string, timestamp int64) (s3.Manifest, bool) {
	key := impl.MakeManifestKey(namespace, timestamp)
	manifest, exists := m.StoredManifests[key]
	return manifest, exists
}

func (m *S3Service) DeleteItem(namespace, filepath string) {
	key := impl.MakeDataKey(namespace, filepath)
	delete(m.StoredItems, key)
}
