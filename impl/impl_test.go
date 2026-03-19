package impl_test

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/valpop/impl"
)

func TestStorageBackends(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Backend Drop-in Replacement Suite")
}

// MockStorage is a simple in-memory implementation of Implementation for testing
type MockStorage struct {
	data       map[string]string // key -> content
	namespaces map[string]bool   // track active namespaces
	closed     bool
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		data:       make(map[string]string),
		namespaces: make(map[string]bool),
		closed:     false,
	}
}

func (m *MockStorage) makeKey(namespace, filepath string, timestamp int64) string {
	return fmt.Sprintf("%s:%d:%s", namespace, timestamp, filepath)
}

func (m *MockStorage) StartPopulate(namespace, bucket string, timestamp int64) error {
	if m.closed {
		return fmt.Errorf("storage is closed")
	}
	m.namespaces[namespace] = true
	return nil
}

func (m *MockStorage) EndPopulate(namespace, bucket string, timestamp int64) error {
	if m.closed {
		return fmt.Errorf("storage is closed")
	}
	// Mock implementation - just track that populate ended
	return nil
}

func (m *MockStorage) SetItem(namespace, filepath, contentType, bucket string, timestamp int64, content string) error {
	if m.closed {
		return fmt.Errorf("storage is closed")
	}
	key := m.makeKey(namespace, filepath, timestamp)
	m.data[key] = content
	return nil
}

func (m *MockStorage) GetItem(namespace, filepath string, timestamp int64) (string, error) {
	if m.closed {
		return "", fmt.Errorf("storage is closed")
	}
	key := m.makeKey(namespace, filepath, timestamp)
	content, exists := m.data[key]
	if !exists {
		return "", nil
	}
	return content, nil
}

func (m *MockStorage) DelKeys(allItems impl.AllItems) error {
	if m.closed {
		return fmt.Errorf("storage is closed")
	}
	for namespace, items := range allItems {
		for filepath, timestamps := range items {
			for _, timestamp := range timestamps {
				key := m.makeKey(namespace, filepath, timestamp)
				delete(m.data, key)
			}
		}
	}
	return nil
}

func (m *MockStorage) PopulateFromDir(namespace, bucket, basepath string, timestamp int64) error {
	if m.closed {
		return fmt.Errorf("storage is closed")
	}
	// Mock implementation - just add some dummy data
	return m.SetItem(namespace, "mock-file.txt", "text/plain", bucket, timestamp, "mock content from "+basepath)
}

func (m *MockStorage) Pop(namespace string, timestamp int64) (impl.AllItems, error) {
	if m.closed {
		return nil, fmt.Errorf("storage is closed")
	}
	result := impl.AllItems{}
	prefix := fmt.Sprintf("%s:%d:", namespace, timestamp)

	for key, _ := range m.data {
		if strings.HasPrefix(key, prefix) {
			// Parse the key to extract namespace, timestamp, filepath
			parts := strings.SplitN(key, ":", 3)
			if len(parts) == 3 {
				if result[namespace] == nil {
					result[namespace] = make(impl.Items)
				}
				filepath := parts[2]
				result[namespace][filepath] = append(result[namespace][filepath], timestamp)
			}
		}
	}

	return result, nil
}

func (m *MockStorage) Close() {
	m.closed = true
	m.data = make(map[string]string)
	m.namespaces = make(map[string]bool)
}

var _ = Describe("Storage Backend Drop-in Replacements", func() {
	var (
		testNamespace = "test-app"
		testBucket    = "test-bucket"
		testTimestamp = int64(1234567890)
	)

	Describe("Drop-in replacement demonstration", func() {
		Context("MockStorage backend", func() {
			var mockBackend *MockStorage

			BeforeEach(func() {
				mockBackend = NewMockStorage()
			})

			AfterEach(func() {
				if mockBackend != nil {
					mockBackend.Close()
				}
			})

			It("should complete a full lifecycle", func() {
				filepath := "index.html"
				content := "<html><body>Test Content</body></html>"

				// Start populate
				err := mockBackend.StartPopulate(testNamespace, testBucket, testTimestamp)
				Expect(err).ToNot(HaveOccurred())

				// Store item
				err = mockBackend.SetItem(testNamespace, filepath, "text/plain", testBucket, testTimestamp, content)
				Expect(err).ToNot(HaveOccurred())

				// Retrieve item
				retrieved, err := mockBackend.GetItem(testNamespace, filepath, testTimestamp)
				Expect(err).ToNot(HaveOccurred())
				Expect(retrieved).To(Equal(content))

				// End populate
				err = mockBackend.EndPopulate(testNamespace, testBucket, testTimestamp)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle multiple items", func() {
				items := map[string]string{
					"index.html": "<html><head><title>Test</title></head><body>Home</body></html>",
					"app.js":     "console.log('App loaded');",
					"style.css":  "body { margin: 0; }",
				}

				// Start populate
				err := mockBackend.StartPopulate(testNamespace, testBucket, testTimestamp)
				Expect(err).ToNot(HaveOccurred())

				// Store multiple items
				for path, content := range items {
					err := mockBackend.SetItem(testNamespace, path, "text/plain", testBucket, testTimestamp, content)
					Expect(err).ToNot(HaveOccurred())
				}

				// Retrieve all items
				for path, expectedContent := range items {
					retrieved, err := mockBackend.GetItem(testNamespace, path, testTimestamp)
					Expect(err).ToNot(HaveOccurred())
					Expect(retrieved).To(Equal(expectedContent))
				}

				// End populate
				err = mockBackend.EndPopulate(testNamespace, testBucket, testTimestamp)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle deletion", func() {
				filepath := "index.html"
				content := "<html><body>Test Content</body></html>"

				// Store some items first
				err := mockBackend.StartPopulate(testNamespace, testBucket, testTimestamp)
				Expect(err).ToNot(HaveOccurred())

				err = mockBackend.SetItem(testNamespace, filepath, "text/plain", testBucket, testTimestamp, content)
				Expect(err).ToNot(HaveOccurred())

				err = mockBackend.EndPopulate(testNamespace, testBucket, testTimestamp)
				Expect(err).ToNot(HaveOccurred())

				// Create deletion data
				allItems := impl.AllItems{
					testNamespace: impl.Items{
						filepath: []int64{testTimestamp},
					},
				}

				// Delete items
				err = mockBackend.DelKeys(allItems)
				Expect(err).ToNot(HaveOccurred())

				// Verify deletion (item should be empty/not found)
				retrieved, err := mockBackend.GetItem(testNamespace, filepath, testTimestamp)
				Expect(err).ToNot(HaveOccurred())
				Expect(retrieved).To(BeEmpty())
			})
		})
	})

	Describe("Interface compatibility", func() {
		It("should allow storage backends to be used interchangeably", func() {
			var backend impl.Implementation

			// Test that we can assign different implementations
			backend = NewMockStorage()
			Expect(backend).ToNot(BeNil())

			// Clean up
			backend.Close()
		})

		It("should demonstrate polymorphic usage", func() {
			backends := []struct {
				name    string
				backend impl.Implementation
			}{
				{"MockStorage", NewMockStorage()},
			}

			for _, b := range backends {
				By(fmt.Sprintf("Testing %s backend", b.name))

				// Each backend should respond to the same interface calls
				err := b.backend.StartPopulate("poly-test", "test-bucket", 999)
				Expect(err).ToNot(HaveOccurred())

				b.backend.Close()
			}
		})
	})

	Describe("Real-world usage scenarios", func() {
		var mockBackend *MockStorage

		BeforeEach(func() {
			mockBackend = NewMockStorage()
		})

		AfterEach(func() {
			if mockBackend != nil {
				mockBackend.Close()
			}
		})

		Context("Complete populate and retrieve workflow", func() {
			It("should handle a deployment with nested paths", func() {
				namespace := "my-web-app"
				timestamp := int64(1700000000)

				// Simulate deploying files with nested paths
				files := map[string]string{
					"index.html":          "<html>Home</html>",
					"assets/css/main.css": "body { margin: 0; }",
					"api/config.json":     `{"version": "1.0.0"}`,
				}

				err := mockBackend.StartPopulate(namespace, testBucket, timestamp)
				Expect(err).ToNot(HaveOccurred())

				for filepath, content := range files {
					err := mockBackend.SetItem(namespace, filepath, "text/plain", testBucket, timestamp, content)
					Expect(err).ToNot(HaveOccurred())
				}

				err = mockBackend.EndPopulate(namespace, testBucket, timestamp)
				Expect(err).ToNot(HaveOccurred())

				// Verify nested path handling
				for filepath, expectedContent := range files {
					retrievedContent, err := mockBackend.GetItem(namespace, filepath, timestamp)
					Expect(err).ToNot(HaveOccurred())
					Expect(retrievedContent).To(Equal(expectedContent))
				}
			})

			It("should handle multiple timestamped deployments", func() {
				namespace := "my-app"
				timestamp1 := int64(1700000000)
				timestamp2 := int64(1700001000)

				// Deploy two versions
				for i, ts := range []int64{timestamp1, timestamp2} {
					err := mockBackend.StartPopulate(namespace, testBucket, ts)
					Expect(err).ToNot(HaveOccurred())
					err = mockBackend.SetItem(namespace, "index.html", "text/html", testBucket, ts, fmt.Sprintf("<html>Version %d</html>", i+1))
					Expect(err).ToNot(HaveOccurred())
					err = mockBackend.EndPopulate(namespace, testBucket, ts)
					Expect(err).ToNot(HaveOccurred())
				}

				// Verify both timestamps are independent
				content1, _ := mockBackend.GetItem(namespace, "index.html", timestamp1)
				content2, _ := mockBackend.GetItem(namespace, "index.html", timestamp2)
				Expect(content1).To(Equal("<html>Version 1</html>"))
				Expect(content2).To(Equal("<html>Version 2</html>"))
			})

			It("should handle cleanup of old deployments", func() {
				namespace := "cleanup-test"

				// Create multiple old deployments
				timestamps := []int64{1700000000, 1700001000, 1700002000}
				for _, ts := range timestamps {
					err := mockBackend.StartPopulate(namespace, testBucket, ts)
					Expect(err).ToNot(HaveOccurred())
					err = mockBackend.SetItem(namespace, "file.txt", "text/plain", testBucket, ts, fmt.Sprintf("Content at %d", ts))
					Expect(err).ToNot(HaveOccurred())
					err = mockBackend.EndPopulate(namespace, testBucket, ts)
					Expect(err).ToNot(HaveOccurred())
				}

				// Delete the oldest two deployments
				allItems := impl.AllItems{
					namespace: impl.Items{
						"file.txt": []int64{timestamps[0], timestamps[1]},
					},
				}

				err := mockBackend.DelKeys(allItems)
				Expect(err).ToNot(HaveOccurred())

				// Verify deleted items are gone
				content, err := mockBackend.GetItem(namespace, "file.txt", timestamps[0])
				Expect(err).ToNot(HaveOccurred())
				Expect(content).To(BeEmpty())

				content, err = mockBackend.GetItem(namespace, "file.txt", timestamps[1])
				Expect(err).ToNot(HaveOccurred())
				Expect(content).To(BeEmpty())

				// Verify the newest deployment still exists
				content, err = mockBackend.GetItem(namespace, "file.txt", timestamps[2])
				Expect(err).ToNot(HaveOccurred())
				Expect(content).To(Equal(fmt.Sprintf("Content at %d", timestamps[2])))
			})
		})

		Context("Error handling scenarios", func() {
			It("should handle operations on closed storage", func() {
				mockBackend.Close()

				namespace := "test"
				timestamp := int64(1700000000)

				// All operations on closed storage should fail
				err := mockBackend.StartPopulate(namespace, testBucket, timestamp)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("closed"))

				err = mockBackend.SetItem(namespace, "file.txt", "text/plain", testBucket, timestamp, "content")
				Expect(err).To(HaveOccurred())

				_, err = mockBackend.GetItem(namespace, "file.txt", timestamp)
				Expect(err).To(HaveOccurred())
			})

			It("should handle retrieval of non-existent items", func() {
				content, err := mockBackend.GetItem("nonexistent", "file.txt", 999)
				Expect(err).ToNot(HaveOccurred())
				Expect(content).To(BeEmpty())
			})

			It("should handle deletion of non-existent items gracefully", func() {
				allItems := impl.AllItems{
					"nonexistent": impl.Items{
						"file.txt": []int64{999, 1000},
					},
				}

				err := mockBackend.DelKeys(allItems)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Multi-namespace scenarios", func() {
			It("should isolate data between different namespaces", func() {
				namespace1 := "app1"
				namespace2 := "app2"
				timestamp := int64(1700000000)

				// Store same file in different namespaces
				err := mockBackend.StartPopulate(namespace1, testBucket, timestamp)
				Expect(err).ToNot(HaveOccurred())
				err = mockBackend.SetItem(namespace1, "config.json", "application/json", testBucket, timestamp, `{"app": "app1"}`)
				Expect(err).ToNot(HaveOccurred())
				err = mockBackend.EndPopulate(namespace1, testBucket, timestamp)
				Expect(err).ToNot(HaveOccurred())

				err = mockBackend.StartPopulate(namespace2, testBucket, timestamp)
				Expect(err).ToNot(HaveOccurred())
				err = mockBackend.SetItem(namespace2, "config.json", "application/json", testBucket, timestamp, `{"app": "app2"}`)
				Expect(err).ToNot(HaveOccurred())
				err = mockBackend.EndPopulate(namespace2, testBucket, timestamp)
				Expect(err).ToNot(HaveOccurred())

				// Verify each namespace has its own data
				content1, err := mockBackend.GetItem(namespace1, "config.json", timestamp)
				Expect(err).ToNot(HaveOccurred())
				Expect(content1).To(Equal(`{"app": "app1"}`))

				content2, err := mockBackend.GetItem(namespace2, "config.json", timestamp)
				Expect(err).ToNot(HaveOccurred())
				Expect(content2).To(Equal(`{"app": "app2"}`))
			})

			It("should handle deletion in one namespace without affecting others", func() {
				namespace1 := "app1"
				namespace2 := "app2"
				timestamp := int64(1700000000)

				// Store data in both namespaces
				for _, ns := range []string{namespace1, namespace2} {
					err := mockBackend.StartPopulate(ns, testBucket, timestamp)
					Expect(err).ToNot(HaveOccurred())
					err = mockBackend.SetItem(ns, "file.txt", "text/plain", testBucket, timestamp, fmt.Sprintf("Content for %s", ns))
					Expect(err).ToNot(HaveOccurred())
					err = mockBackend.EndPopulate(ns, testBucket, timestamp)
					Expect(err).ToNot(HaveOccurred())
				}

				// Delete from namespace1 only
				allItems := impl.AllItems{
					namespace1: impl.Items{
						"file.txt": []int64{timestamp},
					},
				}
				err := mockBackend.DelKeys(allItems)
				Expect(err).ToNot(HaveOccurred())

				// Verify namespace1 is deleted
				content, err := mockBackend.GetItem(namespace1, "file.txt", timestamp)
				Expect(err).ToNot(HaveOccurred())
				Expect(content).To(BeEmpty())

				// Verify namespace2 still exists
				content, err = mockBackend.GetItem(namespace2, "file.txt", timestamp)
				Expect(err).ToNot(HaveOccurred())
				Expect(content).To(Equal(fmt.Sprintf("Content for %s", namespace2)))
			})
		})
	})

	Describe("Common utility functions", func() {
		Context("MakeDataKey", func() {
			It("should generate correct data key format", func() {
				key := impl.MakeDataKey("myapp", "index.html")
				Expect(key).To(Equal("data/myapp/index.html"))
			})

			It("should handle paths with subdirectories", func() {
				key := impl.MakeDataKey("frontend", "assets/js/app.js")
				Expect(key).To(Equal("data/frontend/assets/js/app.js"))
			})

			It("should handle special characters", func() {
				key := impl.MakeDataKey("my-app", "file with spaces.txt")
				Expect(key).To(Equal("data/my-app/file with spaces.txt"))
			})
		})

		Context("MakeManifestKey", func() {
			It("should generate correct manifest key format", func() {
				key := impl.MakeManifestKey("myapp", 1234567890)
				Expect(key).To(Equal("manifests/myapp/1234567890"))
			})

			It("should handle different namespaces", func() {
				key := impl.MakeManifestKey("production-app", 9876543210)
				Expect(key).To(Equal("manifests/production-app/9876543210"))
			})

			It("should handle zero timestamp", func() {
				key := impl.MakeManifestKey("test", 0)
				Expect(key).To(Equal("manifests/test/0"))
			})
		})

		Context("GetContentType", func() {
			It("should return correct content type for HTML", func() {
				contentType := impl.GetContentType("index.html")
				Expect(contentType).To(Equal("text/html; charset=utf-8"))
			})

			It("should return correct content type for CSS", func() {
				contentType := impl.GetContentType("styles.css")
				Expect(contentType).To(Equal("text/css; charset=utf-8"))
			})

			It("should return correct content type for JavaScript", func() {
				contentType := impl.GetContentType("app.js")
				Expect(contentType).To(Equal("application/javascript"))
			})

			It("should return correct content type for WOFF2 fonts", func() {
				contentType := impl.GetContentType("font.woff2")
				Expect(contentType).To(Equal("font/woff2"))
			})

			It("should return correct content type for SVG", func() {
				contentType := impl.GetContentType("logo.svg")
				Expect(contentType).To(Equal("image/svg+xml"))
			})

			It("should return correct content type for JSON", func() {
				contentType := impl.GetContentType("config.json")
				Expect(contentType).To(Equal("application/json"))
			})

			It("should return default for unknown types", func() {
				contentType := impl.GetContentType("file.unknown")
				Expect(contentType).To(Equal("application/octet-stream"))
			})

			It("should handle files without extensions", func() {
				contentType := impl.GetContentType("README")
				Expect(contentType).To(Equal("application/octet-stream"))
			})

			It("should handle paths with directories", func() {
				contentType := impl.GetContentType("assets/styles/main.css")
				Expect(contentType).To(Equal("text/css; charset=utf-8"))
			})
		})

		Context("DetermineManifestsToDelete", func() {
			It("should keep minimum number of manifests regardless of age", func() {
				currentTime := int64(10000)
				manifests := []impl.ManifestInfo{
					{Key: "m1", Timestamp: 1000, Files: []string{"a.txt"}},
					{Key: "m2", Timestamp: 2000, Files: []string{"b.txt"}},
					{Key: "m3", Timestamp: 3000, Files: []string{"c.txt"}},
				}

				// Keep at least 2, timeout 100 - all are old but should keep 2 newest
				toDelete := impl.DetermineManifestsToDelete(manifests, currentTime, 100, 2)
				Expect(len(toDelete)).To(Equal(1))
				Expect(toDelete[0].Key).To(Equal("m1")) // Oldest one
			})

			It("should respect timeout", func() {
				currentTime := int64(10000)
				manifests := []impl.ManifestInfo{
					{Key: "m1", Timestamp: 9000, Files: []string{"a.txt"}}, // Recent
					{Key: "m2", Timestamp: 5000, Files: []string{"b.txt"}}, // Old
					{Key: "m3", Timestamp: 1000, Files: []string{"c.txt"}}, // Very old
				}

				// Keep at least 1, timeout 3000
				toDelete := impl.DetermineManifestsToDelete(manifests, currentTime, 3000, 1)
				Expect(len(toDelete)).To(Equal(2))
				// Should delete m2 and m3 (both older than timeout and beyond minAssetRecords)
			})

			It("should handle empty manifest list", func() {
				currentTime := int64(10000)
				manifests := []impl.ManifestInfo{}

				toDelete := impl.DetermineManifestsToDelete(manifests, currentTime, 1000, 3)
				Expect(len(toDelete)).To(Equal(0))
			})

			It("should keep all if under minimum", func() {
				currentTime := int64(10000)
				manifests := []impl.ManifestInfo{
					{Key: "m1", Timestamp: 1000, Files: []string{"a.txt"}},
					{Key: "m2", Timestamp: 2000, Files: []string{"b.txt"}},
				}

				// minAssetRecords is 5, but only have 2
				toDelete := impl.DetermineManifestsToDelete(manifests, currentTime, 100, 5)
				Expect(len(toDelete)).To(Equal(0))
			})
		})

		Context("SeparateManifests", func() {
			It("should correctly separate manifests into delete and keep", func() {
				currentTime := int64(10000)
				manifests := []impl.ManifestInfo{
					{Key: "m1", Timestamp: 9500, Files: []string{"a.txt"}}, // Keep - recent
					{Key: "m2", Timestamp: 5000, Files: []string{"b.txt"}}, // Delete - old
					{Key: "m3", Timestamp: 1000, Files: []string{"c.txt"}}, // Delete - very old
				}

				toDelete, toKeep := impl.SeparateManifests(manifests, currentTime, 3000, 1)

				Expect(len(toKeep)).To(Equal(1))
				Expect(toKeep[0].Key).To(Equal("m1"))

				Expect(len(toDelete)).To(Equal(2))
				// Should be sorted newest first in results
			})

			It("should prioritize minAssetRecords over timeout", func() {
				currentTime := int64(10000)
				manifests := []impl.ManifestInfo{
					{Key: "m1", Timestamp: 1000, Files: []string{"a.txt"}},
					{Key: "m2", Timestamp: 2000, Files: []string{"b.txt"}},
					{Key: "m3", Timestamp: 3000, Files: []string{"c.txt"}},
				}

				// All are old (timeout=100) but keep 3 minimum
				toDelete, toKeep := impl.SeparateManifests(manifests, currentTime, 100, 3)

				Expect(len(toKeep)).To(Equal(3))
				Expect(len(toDelete)).To(Equal(0))
			})
		})

		Context("DetermineFilesToDelete", func() {
			It("should only delete files not in kept manifests", func() {
				oldManifests := []impl.ManifestInfo{
					{Key: "m1", Timestamp: 1000, Files: []string{"old1.txt", "shared.txt"}},
				}
				keptManifests := []impl.ManifestInfo{
					{Key: "m2", Timestamp: 5000, Files: []string{"shared.txt", "new.txt"}},
				}

				filesToDelete := impl.DetermineFilesToDelete(oldManifests, keptManifests, []string{})

				Expect(filesToDelete).To(ConsistOf("old1.txt"))
				// shared.txt should not be deleted because it's in keptManifests
			})

			It("should protect specified files", func() {
				oldManifests := []impl.ManifestInfo{
					{Key: "m1", Timestamp: 1000, Files: []string{"file1.txt", "protected.json"}},
				}
				keptManifests := []impl.ManifestInfo{}

				filesToDelete := impl.DetermineFilesToDelete(oldManifests, keptManifests, []string{"protected.json"})

				Expect(filesToDelete).To(ConsistOf("file1.txt"))
				// protected.json should not be in the delete list
			})

			It("should handle multiple old and kept manifests", func() {
				oldManifests := []impl.ManifestInfo{
					{Key: "m1", Timestamp: 1000, Files: []string{"a.txt", "b.txt"}},
					{Key: "m2", Timestamp: 2000, Files: []string{"b.txt", "c.txt"}},
				}
				keptManifests := []impl.ManifestInfo{
					{Key: "m3", Timestamp: 5000, Files: []string{"b.txt", "d.txt"}},
				}

				filesToDelete := impl.DetermineFilesToDelete(oldManifests, keptManifests, []string{})

				// a.txt and c.txt are only in old manifests
				// b.txt is in kept manifest so should be protected
				Expect(filesToDelete).To(ConsistOf("a.txt", "c.txt"))
			})

			It("should handle empty manifests", func() {
				oldManifests := []impl.ManifestInfo{}
				keptManifests := []impl.ManifestInfo{}

				filesToDelete := impl.DetermineFilesToDelete(oldManifests, keptManifests, []string{})

				Expect(len(filesToDelete)).To(Equal(0))
			})

			It("should protect fedmods.json style files", func() {
				oldManifests := []impl.ManifestInfo{
					{Key: "m1", Timestamp: 1000, Files: []string{"index.html", "fedmods.json", "config.json"}},
				}
				keptManifests := []impl.ManifestInfo{}

				filesToDelete := impl.DetermineFilesToDelete(oldManifests, keptManifests, []string{"fedmods.json", "config.json"})

				Expect(filesToDelete).To(ConsistOf("index.html"))
			})
		})

		Context("BuildPopulateManifest", func() {
			It("should collect all files and call callback for each", func() {
				// Create a mock filesystem
				mockFS := fstest.MapFS{
					"file1.txt":      {Data: []byte("content1")},
					"file2.js":       {Data: []byte("content2")},
					"dir/file3.html": {Data: []byte("content3")},
				}

				callbackCalls := []impl.FileInfo{}
				manifest, err := impl.BuildPopulateManifest(mockFS, func(file impl.FileInfo) error {
					callbackCalls = append(callbackCalls, file)
					return nil
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(manifest).To(ConsistOf("file1.txt", "file2.js", "dir/file3.html"))
				Expect(len(callbackCalls)).To(Equal(3))

				// Check that content types were set correctly
				for _, call := range callbackCalls {
					if call.Path == "file2.js" {
						Expect(call.ContentType).To(Equal("application/javascript"))
					}
					if call.Path == "dir/file3.html" {
						Expect(call.ContentType).To(Equal("text/html; charset=utf-8"))
					}
				}
			})

			It("should handle empty filesystem", func() {
				mockFS := fstest.MapFS{}

				manifest, err := impl.BuildPopulateManifest(mockFS, func(file impl.FileInfo) error {
					return nil
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(len(manifest)).To(Equal(0))
			})

			It("should propagate callback errors", func() {
				mockFS := fstest.MapFS{
					"file1.txt": {Data: []byte("content1")},
				}

				expectedErr := fmt.Errorf("callback error")
				manifest, err := impl.BuildPopulateManifest(mockFS, func(file impl.FileInfo) error {
					return expectedErr
				})

				Expect(err).To(Equal(expectedErr))
				// Manifest will contain the file since it's added before callback error
				Expect(len(manifest)).To(Equal(1))
			})

			It("should skip directories", func() {
				mockFS := fstest.MapFS{
					"dir/file.txt": {Data: []byte("content")},
				}

				callCount := 0
				manifest, err := impl.BuildPopulateManifest(mockFS, func(file impl.FileInfo) error {
					callCount++
					return nil
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(manifest).To(ConsistOf("dir/file.txt"))
				Expect(callCount).To(Equal(1)) // Only called for file, not directory
			})

			It("should handle nested directory structures", func() {
				mockFS := fstest.MapFS{
					"a/b/c/deep.txt": {Data: []byte("deep content")},
					"a/shallow.txt":  {Data: []byte("shallow content")},
					"root.txt":       {Data: []byte("root content")},
				}

				manifest, err := impl.BuildPopulateManifest(mockFS, func(file impl.FileInfo) error {
					return nil
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(manifest).To(ConsistOf("a/b/c/deep.txt", "a/shallow.txt", "root.txt"))
			})

			It("should provide correct file content to callback", func() {
				expectedContent := "test content 123"
				mockFS := fstest.MapFS{
					"test.txt": {Data: []byte(expectedContent)},
				}

				var capturedContent string
				_, err := impl.BuildPopulateManifest(mockFS, func(file impl.FileInfo) error {
					capturedContent = file.Content
					return nil
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(capturedContent).To(Equal(expectedContent))
			})
		})

		Context("ParseManifest", func() {
			It("should parse old format manifest (array)", func() {
				jsonData := []byte(`["file1.txt", "file2.js", "dir/file3.html"]`)

				manifest, err := impl.ParseManifest(jsonData)

				Expect(err).ToNot(HaveOccurred())
				Expect(manifest.Files).To(Equal([]string{"file1.txt", "file2.js", "dir/file3.html"}))
				Expect(manifest.Image).To(Equal(""))
				Expect(manifest.Timestamp).To(Equal(int64(0)))
			})

			It("should parse new format manifest (object)", func() {
				jsonData := []byte(`{"files": ["file1.txt", "file2.js"], "image": "myapp:v1.2.3", "timestamp": 1742472000}`)

				manifest, err := impl.ParseManifest(jsonData)

				Expect(err).ToNot(HaveOccurred())
				Expect(manifest.Files).To(Equal([]string{"file1.txt", "file2.js"}))
				Expect(manifest.Image).To(Equal("myapp:v1.2.3"))
				Expect(manifest.Timestamp).To(Equal(int64(1742472000)))
			})

			It("should handle empty manifest (old format)", func() {
				jsonData := []byte(`[]`)

				manifest, err := impl.ParseManifest(jsonData)

				Expect(err).ToNot(HaveOccurred())
				Expect(len(manifest.Files)).To(Equal(0))
			})

			It("should handle empty manifest (new format)", func() {
				jsonData := []byte(`{"files": [], "image": "", "timestamp": 0}`)

				manifest, err := impl.ParseManifest(jsonData)

				Expect(err).ToNot(HaveOccurred())
				Expect(len(manifest.Files)).To(Equal(0))
			})

			It("should return error for invalid JSON", func() {
				jsonData := []byte(`{invalid json}`)

				manifest, err := impl.ParseManifest(jsonData)

				Expect(err).To(HaveOccurred())
				Expect(manifest.Files).To(BeNil())
			})

			It("should handle manifest with special characters in filenames (old format)", func() {
				jsonData := []byte(`["file with spaces.txt", "file-with-dashes.js", "file_underscore.html"]`)

				manifest, err := impl.ParseManifest(jsonData)

				Expect(err).ToNot(HaveOccurred())
				Expect(manifest.Files).To(ConsistOf("file with spaces.txt", "file-with-dashes.js", "file_underscore.html"))
			})

			It("should handle manifest with special characters in filenames (new format)", func() {
				jsonData := []byte(`{"files": ["file with spaces.txt", "file-with-dashes.js"], "image": "test:v1", "timestamp": 123}`)

				manifest, err := impl.ParseManifest(jsonData)

				Expect(err).ToNot(HaveOccurred())
				Expect(manifest.Files).To(ConsistOf("file with spaces.txt", "file-with-dashes.js"))
			})
		})
	})
})
