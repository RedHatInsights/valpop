package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	minio "github.com/minio/minio-go/v7"
	creds "github.com/minio/minio-go/v7/pkg/credentials"
)

type Minio struct {
	ctx    context.Context
	client *minio.Client
}

func NewMinio(addr, username, password string) (Minio, error) {
	client, err := minio.New(addr, &minio.Options{
		Creds:  creds.NewStaticV4(username, password, ""),
		Secure: false, // Change to `true` if using HTTPS
	})
	if err != nil {
		panic(err)
	}
	return Minio{
		ctx:    context.Background(),
		client: client,
	}, nil
}

func makeDataPath(namespace, filepath string, _ int64) string {
	return fmt.Sprintf("data/%s/%s", namespace, filepath)
}
func makeManifestPath(namespace string, timestamp int64) string {
	return fmt.Sprintf("manifests/%s/%d", namespace, timestamp)
}

func (m *Minio) Close() {
}

func (m *Minio) StartPopulate(namespace string, timestamp int64) error {
	return nil
}

func (m *Minio) EndPopulate(namespace string, timestamp int64) error {
	return nil
}

func (m *Minio) SetItem(namespace, filepath string, timestamp int64, contents string) error {
	key := makeDataPath(namespace, filepath, timestamp)

	fmt.Printf("Uploading: %s: %s (%d)\n", filepath, key, len(contents))

	_, err := m.client.FPutObject(m.ctx, "frontend", key, filepath, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("err from s3:%w", err)
	}
	return nil
}

func (m *Minio) SetManifest(namespace string, timestamp int64, files Manifest) error {
	key := makeManifestPath(namespace, timestamp)

	fmt.Printf("manifest %s: %d (%d)\n", key, len(files), timestamp)
	raw, err := json.Marshal(files)
	if err != nil {
		return fmt.Errorf("could not encode manifest:%w", err)
	}

	_, err = m.client.PutObject(m.ctx, "frontend", key, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("err from s3:%w", err)
	}
	return nil
}

type Manifest []string

func PopulateFn(addr, source, prefix, username, password string, timeout int64) error {
	currentTime := time.Now().Unix()

	client, err := NewMinio(addr, username, password)
	if err != nil {
		return err
	}

	defer client.Close()
	fileSystem := os.DirFS(source)
	client.StartPopulate(prefix, currentTime)
	manifest := Manifest{}
	err = fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		contents, werr := os.ReadFile(path)
		if werr != nil {
			return werr
		}
		manifest = append(manifest, path)
		return client.SetItem(prefix, path, currentTime, string(contents))
	})
	if err != nil {
		fmt.Printf("%v", err)
	}

	err = client.SetManifest(prefix, currentTime, manifest)
	if err != nil {
		return err
	}

	err = client.EndPopulate(prefix, currentTime)
	if err != nil {
		return err
	}

	return client.CleanupCache(prefix, timeout)
}

func (m *Minio) CleanupCache(prefix string, timeout int64) error {
	currentTime := time.Now().Unix()
	bucketPrefix := "manifests/" + prefix + "/"
	oldFiles := map[string]bool{}
	newFiles := map[string]bool{}
	oldManifests := []string{}
	for object := range m.client.ListObjects(m.ctx, "frontend", minio.ListObjectsOptions{Prefix: bucketPrefix, Recursive: true}) {
		timestampString, _ := strings.CutPrefix(object.Key, "manifests/"+prefix+"/")
		timestamp, err := strconv.Atoi(timestampString)
		if err != nil {
			return fmt.Errorf("could not get timestamp: %w", err)
		}
		if currentTime-int64(timestamp) > timeout {
			manifest, err := m.getManifest(object.Key)
			if err != nil {
				return fmt.Errorf("could not get manifest: %w", err)
			}

			for _, file := range manifest {
				oldFiles[file] = true
			}
			oldManifests = append(oldManifests, object.Key)
		} else {
			manifest, err := m.getManifest(object.Key)
			if err != nil {
				return fmt.Errorf("could not get manifest: %w", err)
			}

			for _, file := range manifest {
				newFiles[file] = true
			}
		}
	}
	for file := range newFiles {
		fmt.Printf("Protecting: %s\n", file)
		delete(oldFiles, file)
	}
	delete(oldFiles, "fedmods.json")

	for file := range oldFiles {
		err := m.client.RemoveObject(m.ctx, "frontend", makeDataPath(prefix, file, currentTime), minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("unable to remove object: %w", err)
		}
		fmt.Printf("Removed file %s\n", file)
	}

	for _, file := range oldManifests {
		err := m.client.RemoveObject(m.ctx, "frontend", file, minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("unable to remove object: %w", err)
		}
		fmt.Printf("Removed manifest %s\n", file)
	}

	return nil
}

func (m *Minio) getManifest(key string) (Manifest, error) {
	obj, err := m.client.GetObject(m.ctx, "frontend", key, minio.GetObjectOptions{})
	if err != nil {
		return Manifest{}, fmt.Errorf("could not get object: %w", err)
	}

	manifest := Manifest{}
	rawData, err := io.ReadAll(obj)
	if err != nil {
		return Manifest{}, fmt.Errorf("could not read object: %w", err)
	}

	err = json.Unmarshal(rawData, &manifest)
	if err != nil {
		return Manifest{}, fmt.Errorf("could not unmarshal object: %w", err)
	}
	return manifest, nil
}
