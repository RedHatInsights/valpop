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

func (m *Minio) SetItem(namespace, filepath, contentType, bucket string, timestamp int64, contents string, cacheMaxAge int64) error {
	key := impl.MakeDataKey(namespace, filepath)
	content_len := len(contents)

	fmt.Printf("Uploading: %s: %s (%d)\n", filepath, key, content_len)

	cacheControl := getCacheControl(filepath, cacheMaxAge)
	_, err := m.client.PutObject(m.ctx, bucket, key, bytes.NewReader([]byte(contents)), int64(content_len), minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: cacheControl,
	})
	if err != nil {
		return fmt.Errorf("err from s3:%w", err)
	}
	return nil
}

func (m *Minio) SetManifest(namespace, bucket string, timestamp int64, manifest impl.Manifest) error {
	key := impl.MakeManifestKey(namespace, timestamp)

	fmt.Printf("manifest %s: %d files, image: %s, timestamp: %d\n", key, len(manifest.Files), manifest.Image, manifest.Timestamp)
	raw, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("could not encode manifest:%w", err)
	}

	_, err = m.client.PutObject(m.ctx, bucket, key, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("err from s3:%w", err)
	}
	return nil
}


func (m *Minio) PopulateFn(addr, bucket, source, prefix, image string, timeout int64, minAssetRecords int64, cacheMaxAge int64) error {
	currentTime := time.Now().Unix()

	// Check if latest manifest has the same image to avoid duplicate uploads
	latestManifest, err := m.getLatestManifest(prefix, bucket)
	if err == nil && latestManifest.Image != "" && latestManifest.Image == image {
		fmt.Printf("Skipping upload: image %s already exists in latest manifest\n", image)
		return nil
	}

	fileSystem := os.DirFS(source)
	m.StartPopulate(prefix, bucket, currentTime)

	// Use common business logic to walk filesystem and collect files
	fileList, err := impl.BuildPopulateManifest(fileSystem, func(file impl.FileInfo) error {
		fmt.Printf("Finding file: %s\n", file.Path)
		return m.SetItem(prefix, file.Path, file.ContentType, bucket, currentTime, file.Content, cacheMaxAge)
	})
	if err != nil {
		fmt.Printf("%v", err)
		return err
	}

	manifest := impl.Manifest{
		Files:     fileList,
		Image:     image,
		Timestamp: currentTime,
	}

	err = m.SetManifest(prefix, bucket, currentTime, manifest)
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
			Files:     manifestData.Files,
		})
	}

	// Use common logic to determine what to delete
	toDelete, toKeep := impl.SeparateManifests(allManifests, currentTime, timeout, minAssetRecords)

	// Determine which files to delete
	filesToDelete := impl.DetermineFilesToDelete(toDelete, toKeep, []string{"fed-mods.json"})

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

func getCacheControl(filepath string, cacheMaxAge int64) string {
	// Short cache to protect origin
	// Adding stale-while-revalidate allows CDN to serve the stale file (up to the specified seconds) while new file is fetched in the background
	// Requests after max-age and stale-while-revalidate will be treated as a cache miss
	if strings.HasSuffix(filepath, "index.html") ||
		strings.HasSuffix(filepath, "fed-mods.json") ||
		strings.HasSuffix(filepath, "app-info.json") ||
		strings.HasSuffix(filepath, "app-info.deps.json") {
		return "public, max-age=60, stale-while-revalidate=300"
	}

	// All other assets use the configured (or default) cache max-age value
	return fmt.Sprintf("public, max-age=%d", cacheMaxAge)
}

func (m *Minio) getLatestManifest(prefix, bucket string) (impl.Manifest, error) {
	bucketPrefix := "manifests/" + prefix + "/"

	var latestManifest impl.Manifest
	var latestTimestamp int64 = 0

	for object := range m.client.ListObjects(m.ctx, bucket, minio.ListObjectsOptions{Prefix: bucketPrefix, Recursive: true}) {
		timestampString, _ := strings.CutPrefix(object.Key, "manifests/"+prefix+"/")
		timestamp, err := strconv.Atoi(timestampString)
		if err != nil {
			continue
		}

		if int64(timestamp) > latestTimestamp {
			manifest, err := m.getManifest(object.Key, bucket)
			if err != nil {
				continue
			}
			latestManifest = manifest
			latestTimestamp = int64(timestamp)
		}
	}

	if latestTimestamp == 0 {
		return impl.Manifest{}, fmt.Errorf("no manifests found")
	}

	return latestManifest, nil
}

func (m *Minio) getManifest(key, bucket string) (impl.Manifest, error) {
	obj, err := m.client.GetObject(m.ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return impl.Manifest{}, fmt.Errorf("could not get object: %w", err)
	}

	rawData, err := io.ReadAll(obj)
	if err != nil {
		return impl.Manifest{}, fmt.Errorf("could not read object: %w", err)
	}

	// Use common business logic to parse manifest
	manifestData, err := impl.ParseManifest(rawData)
	if err != nil {
		return impl.Manifest{}, err
	}

	return manifestData, nil
}
