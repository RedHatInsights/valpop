package impl

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// AllItems maps namespace to Items
type AllItems map[string]Items

// Items maps filepath to list of timestamps
type Items map[string][]int64

// Implementation defines the common interface for all storage implementations
type Implementation interface {
	// Lifecycle operations
	StartPopulate(namespace, bucket string, timestamp int64) error
	EndPopulate(namespace, bucket string, timestamp int64) error
	Close()

	// Item operations
	SetItem(namespace, filepath, contentType, bucket string, timestamp int64, content string) error
	GetItem(namespace, filepath string, timestamp int64) (string, error)
	DelKeys(allItems AllItems) error
	PopulateFromDir(namespace, bucket, basepath string, timestamp int64) error
	Pop(namespace string, timestamp int64) (AllItems, error)
}

// MakeDataKey generates a consistent key format for data items
// Format: data/{namespace}/{filepath}
func MakeDataKey(namespace, filepath string) string {
	return fmt.Sprintf("data/%s/%s", namespace, filepath)
}

// MakeManifestKey generates a consistent key format for manifests
// Format: manifests/{namespace}/{timestamp}
func MakeManifestKey(namespace string, timestamp int64) string {
	return fmt.Sprintf("manifests/%s/%d", namespace, timestamp)
}

func GetContentType(filepath string) string {
	switch {
	case strings.HasSuffix(filepath, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(filepath, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(filepath, ".js"):
		return "application/javascript"
	case strings.HasSuffix(filepath, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(filepath, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(filepath, ".png"):
		return "image/png"
	case strings.HasSuffix(filepath, ".jpeg") || strings.HasSuffix(filepath, ".jpg"):
		return "image/jpeg"
	case strings.HasSuffix(filepath, ".json"):
		return "application/json"
	}
	return "application/octet-stream"
}

// ManifestInfo represents a manifest with its metadata
type ManifestInfo struct {
	Key       string
	Timestamp int64
	Files     []string
}

// DetermineManifestsToDelete returns manifests that should be deleted based on retention policy
// Only keeps the N most recent manifests based on minAssetRecords
func DetermineManifestsToDelete(allManifests []ManifestInfo, currentTime, timeout, minAssetRecords int64) []ManifestInfo {
	// Sort manifests by timestamp (newest first)
	sorted := make([]ManifestInfo, len(allManifests))
	copy(sorted, allManifests)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp > sorted[j].Timestamp
	})

	toDelete := []ManifestInfo{}
	for i, manifest := range sorted {
		// Keep only the N most recent manifests (minAssetRecords)
		// timeout parameter is ignored - only minAssetRecords matters
		if int64(i) >= minAssetRecords {
			toDelete = append(toDelete, manifest)
		}
	}

	return toDelete
}

// DetermineFilesToDelete returns files that should be deleted
// Files are only deleted if they're in old manifests but not in any kept manifests
func DetermineFilesToDelete(oldManifests, keptManifests []ManifestInfo, protectedFiles []string) []string {
	oldFiles := make(map[string]bool)
	newFiles := make(map[string]bool)

	// Collect files from old manifests
	for _, manifest := range oldManifests {
		for _, file := range manifest.Files {
			oldFiles[file] = true
		}
	}

	// Collect files from kept manifests
	for _, manifest := range keptManifests {
		for _, file := range manifest.Files {
			newFiles[file] = true
		}
	}

	// Protect files that are still referenced in newer manifests
	for file := range newFiles {
		delete(oldFiles, file)
	}

	// Protect specific files
	for _, file := range protectedFiles {
		delete(oldFiles, file)
	}

	// Convert to slice
	filesToDelete := make([]string, 0, len(oldFiles))
	for file := range oldFiles {
		filesToDelete = append(filesToDelete, file)
	}

	return filesToDelete
}

// SeparateManifests separates manifests into those to delete and those to keep
// Only keeps the N most recent manifests based on minAssetRecords
func SeparateManifests(allManifests []ManifestInfo, currentTime, timeout, minAssetRecords int64) (toDelete, toKeep []ManifestInfo) {
	// Sort manifests by timestamp (newest first)
	sorted := make([]ManifestInfo, len(allManifests))
	copy(sorted, allManifests)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp > sorted[j].Timestamp
	})

	toDelete = []ManifestInfo{}
	toKeep = []ManifestInfo{}

	for i, manifest := range sorted {
		// Keep only the N most recent manifests (minAssetRecords)
		// timeout parameter is ignored - only minAssetRecords matters
		if int64(i) >= minAssetRecords {
			toDelete = append(toDelete, manifest)
		} else {
			toKeep = append(toKeep, manifest)
		}
	}

	return toDelete, toKeep
}

// FileSystemReader abstracts file system operations for testing
type FileSystemReader interface {
	WalkDir(root string, fn fs.WalkDirFunc) error
	ReadFile(name string) ([]byte, error)
}

// FileInfo represents a file discovered during population
type FileInfo struct {
	Path        string
	Content     string
	ContentType string
}

// BuildPopulateManifest walks a filesystem and collects files into a manifest
// This is the core business logic for populate operations, independent of storage implementation
func BuildPopulateManifest(fileSystem fs.FS, callback func(FileInfo) error) ([]string, error) {
	manifest := []string{}

	err := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		contents, readErr := fs.ReadFile(fileSystem, path)
		if readErr != nil {
			return readErr
		}

		manifest = append(manifest, path)
		contentType := GetContentType(path)

		// Call the callback with file info
		return callback(FileInfo{
			Path:        path,
			Content:     string(contents),
			ContentType: contentType,
		})
	})

	return manifest, err
}

// Manifest represents the structure of a manifest
type Manifest struct {
	Files       []string `json:"files"`
	Image       string   `json:"image"`
	ValpopImage string   `json:"valpopImage,omitempty"`
	Timestamp   int64    `json:"timestamp"`
}

// ParseManifest unmarshals a manifest from JSON bytes
// Returns the manifest data, handling both old (array) and new (object) formats
func ParseManifest(rawData []byte) (Manifest, error) {
	// Trim whitespace to check format
	trimmed := strings.TrimSpace(string(rawData))

	// Check if it's the new object format (starts with '{')
	if strings.HasPrefix(trimmed, "{") {
		var manifest Manifest
		err := json.Unmarshal(rawData, &manifest)
		if err != nil {
			return Manifest{}, fmt.Errorf("could not unmarshal manifest: %w", err)
		}
		return manifest, nil
	}

	// Fall back to old format (array of files, starts with '[')
	var files []string
	err := json.Unmarshal(rawData, &files)
	if err != nil {
		return Manifest{}, fmt.Errorf("could not unmarshal manifest: %w", err)
	}

	// Convert old format to new format
	return Manifest{
		Files:     files,
		Image:     "",
		Timestamp: 0,
	}, nil
}
