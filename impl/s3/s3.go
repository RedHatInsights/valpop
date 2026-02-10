package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	impl "github.com/RedHatInsights/valpop/impl"
	minio "github.com/minio/minio-go/v7"
	creds "github.com/minio/minio-go/v7/pkg/credentials"
)

type Minio struct {
	ctx    context.Context
	client S3Client
}

// NewMinio creates a new Minio instance with a real MinIO client
func NewMinio(addr, username, password string) (Minio, error) {
	client, err := minio.New(addr, &minio.Options{
		Creds:  creds.NewStaticV4(username, password, ""),
		Secure: false, // Change to `true` if using HTTPS
	})
	if err != nil {
		return Minio{}, fmt.Errorf("failed to create S3 client: %w", err)
	}
	return NewMinioWithClient(client), nil
}

// NewMinioWithClient creates a new Minio instance with a custom S3Client
// This allows for dependency injection of mock clients for testing
func NewMinioWithClient(client S3Client) Minio {
	return Minio{
		ctx:    context.Background(),
		client: client,
	}
}

func (m *Minio) Close() {
}

func (m *Minio) StartPopulate(namespace, bucket string, timestamp int64) error {
	return nil
}

func (m *Minio) EndPopulate(namespace, bucket string, timestamp int64) error {
	return nil
}

func (m *Minio) SetItem(namespace, filepath, contentType, bucket string, timestamp int64, contents string) error {
	key := impl.MakeDataKey(namespace, filepath)
	content_len := len(contents)

	fmt.Printf("Uploading: %s: %s (%d)\n", filepath, key, content_len)

	_, err := m.client.PutObject(m.ctx, bucket, key, bytes.NewReader([]byte(contents)), int64(content_len), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("err from s3:%w", err)
	}
	return nil
}

func (m *Minio) SetManifest(namespace, bucket string, timestamp int64, files Manifest) error {
	key := impl.MakeManifestKey(namespace, timestamp)

	fmt.Printf("manifest %s: %d (%d)\n", key, len(files), timestamp)
	raw, err := json.Marshal(files)
	if err != nil {
		return fmt.Errorf("could not encode manifest:%w", err)
	}

	_, err = m.client.PutObject(m.ctx, bucket, key, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("err from s3:%w", err)
	}
	return nil
}

type Manifest []string

func (m *Minio) PopulateFn(addr, bucket, source, prefix string, timeout int64, minAssetRecords int64) error {
	currentTime := time.Now().Unix()

	fileSystem := os.DirFS(source)
	m.StartPopulate(prefix, bucket, currentTime)

	// Use common business logic to walk filesystem and collect files
	manifest, err := impl.BuildPopulateManifest(fileSystem, func(file impl.FileInfo) error {
		fmt.Printf("Finding file: %s\n", file.Path)
		return m.SetItem(prefix, file.Path, file.ContentType, bucket, currentTime, file.Content)
	})
	if err != nil {
		fmt.Printf("%v", err)
		return err
	}

	err = m.SetManifest(prefix, bucket, currentTime, Manifest(manifest))
	if err != nil {
		return err
	}

	err = m.EndPopulate(prefix, bucket, currentTime)
	if err != nil {
		return err
	}

	return m.CleanupCache(prefix, bucket, timeout, minAssetRecords)
}

func (m *Minio) CleanupCache(prefix, bucket string, timeout int64, minAssetRecords int64) error {
	currentTime := time.Now().Unix()
	bucketPrefix := "manifests/" + prefix + "/"

	// Collect all manifests with their timestamps
	allManifests := []impl.ManifestInfo{}

	for object := range m.client.ListObjects(m.ctx, bucket, minio.ListObjectsOptions{Prefix: bucketPrefix, Recursive: true}) {
		timestampString, _ := strings.CutPrefix(object.Key, "manifests/"+prefix+"/")
		timestamp, err := strconv.Atoi(timestampString)
		if err != nil {
			return fmt.Errorf("could not get timestamp: %w", err)
		}

		// Get manifest contents
		manifestData, err := m.getManifest(object.Key, bucket)
		if err != nil {
			return fmt.Errorf("could not get manifest: %w", err)
		}

		allManifests = append(allManifests, impl.ManifestInfo{
			Key:       object.Key,
			Timestamp: int64(timestamp),
			Files:     manifestData,
		})
	}

	// Use common logic to determine what to delete
	toDelete, toKeep := impl.SeparateManifests(allManifests, currentTime, timeout, minAssetRecords)

	// Determine which files to delete
	filesToDelete := impl.DetermineFilesToDelete(toDelete, toKeep, []string{"fedmods.json"})

	// Remove old files
	for _, file := range filesToDelete {
		err := m.client.RemoveObject(m.ctx, bucket, impl.MakeDataKey(prefix, file), minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("unable to remove object: %w", err)
		}
		fmt.Printf("Removed file %s\n", file)
	}

	// Remove old manifests
	for _, manifest := range toDelete {
		err := m.client.RemoveObject(m.ctx, bucket, manifest.Key, minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("unable to remove object: %w", err)
		}
		fmt.Printf("Removed manifest %s\n", manifest.Key)
	}

	return nil
}

func (m *Minio) getManifest(key, bucket string) (Manifest, error) {
	obj, err := m.client.GetObject(m.ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return Manifest{}, fmt.Errorf("could not get object: %w", err)
	}

	rawData, err := io.ReadAll(obj)
	if err != nil {
		return Manifest{}, fmt.Errorf("could not read object: %w", err)
	}

	// Use common business logic to parse manifest
	files, err := impl.ParseManifest(rawData)
	if err != nil {
		return Manifest{}, err
	}

	return Manifest(files), nil
}
