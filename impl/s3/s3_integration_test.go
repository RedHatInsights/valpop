package s3_test

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/valpop/impl"
	"github.com/RedHatInsights/valpop/impl/mock"
)

var _ = Describe("S3 Integration Tests", func() {
	var mockService *mock.S3Service

	BeforeEach(func() {
		mockService = mock.NewS3Service()
	})

	Describe("Real-world application deployment scenario", func() {
		Context("Frontend application deployment", func() {
			It("should handle complete deployment workflow", func() {
				namespace := "my-frontend-app"
				bucket := "production-frontend"
				timestamp := time.Now().Unix()

				// Start deployment
				err := mockService.StartPopulate(namespace, bucket, timestamp)
				Expect(err).ToNot(HaveOccurred())

				// Deploy frontend assets
				assets := map[string]string{
					"index.html":          "<html><head><title>My App</title></head><body><div id='root'></div></body></html>",
					"static/css/main.css": "body { font-family: Arial; margin: 0; padding: 20px; }",
					"static/js/app.js":    "console.log('Application starting...'); ReactDOM.render(App, document.getElementById('root'));",
					"static/js/vendor.js": "// React and other vendor libraries would be here",
					"assets/logo.png":     "binary-png-data-would-be-here",
					"assets/favicon.ico":  "binary-ico-data-would-be-here",
					"manifest.json":       `{"name": "My App", "short_name": "MyApp", "start_url": "/"}`,
					"robots.txt":          "User-agent: *\nDisallow:",
				}

				// Store all assets
				for filepath, content := range assets {
					err := mockService.SetItem(namespace, filepath, "text/plain", bucket, timestamp, content)
					Expect(err).ToNot(HaveOccurred())
				}

				// Create and store manifest
				manifestFiles := []string{}
				for filepath := range assets {
					manifestFiles = append(manifestFiles, filepath)
				}
				manifest := impl.Manifest{
					Files:     manifestFiles,
					Image:     "test-image:v1",
					Timestamp: timestamp,
				}
				err = mockService.SetManifest(namespace, bucket, timestamp, manifest)
				Expect(err).ToNot(HaveOccurred())

				// End deployment
				err = mockService.EndPopulate(namespace, bucket, timestamp)
				Expect(err).ToNot(HaveOccurred())

				// Verify all assets were stored
				for filepath, expectedContent := range assets {
					storedContent, exists := mockService.GetStoredItem(namespace, filepath)
					Expect(exists).To(BeTrue(), fmt.Sprintf("Asset %s should exist", filepath))
					Expect(storedContent).To(Equal(expectedContent), fmt.Sprintf("Content mismatch for %s", filepath))
				}

				// Verify manifest was stored
				storedManifest, exists := mockService.GetStoredManifest(namespace, timestamp)
				Expect(exists).To(BeTrue())
				Expect(len(storedManifest.Files)).To(Equal(len(assets)))

				// Verify all expected operations were performed
				expectedOps := []string{"StartPopulate"}
				for range assets {
					expectedOps = append(expectedOps, "SetItem")
				}
				expectedOps = append(expectedOps, "SetManifest", "EndPopulate")

				Expect(len(mockService.Operations)).To(Equal(len(expectedOps)))
				Expect(mockService.Operations[0]).To(Equal("StartPopulate"))
				Expect(mockService.Operations[len(mockService.Operations)-2]).To(Equal("SetManifest"))
				Expect(mockService.Operations[len(mockService.Operations)-1]).To(Equal("EndPopulate"))
			})
		})

		Context("Multiple deployment versions", func() {
			It("should handle multiple versions and cleanup old ones", func() {
				namespace := "versioned-app"
				bucket := "staging-frontend"

				// Deploy version 1.0
				v1Timestamp := time.Now().Unix() - 7200 // 2 hours ago
				v1Assets := map[string]string{
					"index.html": "<html><body><h1>Version 1.0</h1></body></html>",
					"app.js":     "console.log('v1.0');",
				}

				for filepath, content := range v1Assets {
					err := mockService.SetItem(namespace, filepath, "text/plain", bucket, v1Timestamp, content)
					Expect(err).ToNot(HaveOccurred())
				}

				v1Manifest := impl.Manifest{
					Files:     []string{"index.html", "app.js"},
					Image:     "test-image:v1.0",
					Timestamp: v1Timestamp,
				}
				err := mockService.SetManifest(namespace, bucket, v1Timestamp, v1Manifest)
				Expect(err).ToNot(HaveOccurred())

				// Deploy version 2.0
				v2Timestamp := time.Now().Unix() - 3600 // 1 hour ago
				v2Assets := map[string]string{
					"index.html":     "<html><body><h1>Version 2.0</h1></body></html>",
					"app.js":         "console.log('v2.0');",
					"new-feature.js": "console.log('New feature!');",
				}

				for filepath, content := range v2Assets {
					err := mockService.SetItem(namespace, filepath, "text/plain", bucket, v2Timestamp, content)
					Expect(err).ToNot(HaveOccurred())
				}

				v2Manifest := impl.Manifest{
					Files:     []string{"index.html", "app.js", "new-feature.js"},
					Image:     "test-image:v2.0",
					Timestamp: v2Timestamp,
				}
				err = mockService.SetManifest(namespace, bucket, v2Timestamp, v2Manifest)
				Expect(err).ToNot(HaveOccurred())

				// Deploy version 3.0 (current)
				v3Timestamp := time.Now().Unix() - 300 // 5 minutes ago
				v3Assets := map[string]string{
					"index.html":         "<html><body><h1>Version 3.0</h1></body></html>",
					"app.js":             "console.log('v3.0');",
					"new-feature.js":     "console.log('Enhanced feature!');",
					"another-feature.js": "console.log('Another feature!');",
				}

				for filepath, content := range v3Assets {
					err := mockService.SetItem(namespace, filepath, "text/plain", bucket, v3Timestamp, content)
					Expect(err).ToNot(HaveOccurred())
				}

				v3Manifest := impl.Manifest{
					Files:     []string{"index.html", "app.js", "new-feature.js", "another-feature.js"},
					Image:     "test-image:v3.0",
					Timestamp: v3Timestamp,
				}
				err = mockService.SetManifest(namespace, bucket, v3Timestamp, v3Manifest)
				Expect(err).ToNot(HaveOccurred())

				// Verify all versions are stored
				_, exists := mockService.GetStoredManifest(namespace, v1Timestamp)
				Expect(exists).To(BeTrue())
				_, exists = mockService.GetStoredManifest(namespace, v2Timestamp)
				Expect(exists).To(BeTrue())
				_, exists = mockService.GetStoredManifest(namespace, v3Timestamp)
				Expect(exists).To(BeTrue())

				// Cleanup old versions (keep only 2 versions, 30-minute timeout)
				timeout := int64(1800) // 30 minutes
				minAssetRecords := int64(2)
				err = mockService.CleanupCache(namespace, bucket, timeout, minAssetRecords)
				Expect(err).ToNot(HaveOccurred())

				// Verify cleanup results
				_, exists = mockService.GetStoredManifest(namespace, v1Timestamp)
				Expect(exists).To(BeFalse(), "Version 1.0 should be deleted (too old)")
				_, exists = mockService.GetStoredManifest(namespace, v2Timestamp)
				Expect(exists).To(BeTrue(), "Version 2.0 should remain")
				_, exists = mockService.GetStoredManifest(namespace, v3Timestamp)
				Expect(exists).To(BeTrue(), "Version 3.0 should remain")
			})
		})
	})

	Describe("Error resilience scenarios", func() {
		Context("Storage failures", func() {
			It("should handle storage failure during deployment", func() {
				namespace := "failing-app"
				bucket := "test-bucket"
				timestamp := time.Now().Unix()

				// Start deployment successfully
				err := mockService.StartPopulate(namespace, bucket, timestamp)
				Expect(err).ToNot(HaveOccurred())

				// Store first asset successfully
				err = mockService.SetItem(namespace, "index.html", "text/plain", bucket, timestamp, "<html></html>")
				Expect(err).ToNot(HaveOccurred())

				// Simulate storage failure
				mockService.Errors["SetItem"] = fmt.Errorf("disk full")

				// Attempt to store second asset - should fail
				err = mockService.SetItem(namespace, "app.js", "text/plain", bucket, timestamp, "console.log('app');")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("disk full"))

				// Verify first asset is still stored
				content, exists := mockService.GetStoredItem(namespace, "index.html")
				Expect(exists).To(BeTrue())
				Expect(content).To(Equal("<html></html>"))

				// Verify second asset was not stored
				_, exists = mockService.GetStoredItem(namespace, "app.js")
				Expect(exists).To(BeFalse())
			})

			It("should handle manifest storage failure", func() {
				namespace := "manifest-fail-app"
				bucket := "test-bucket"
				timestamp := time.Now().Unix()

				// Store assets successfully
				err := mockService.SetItem(namespace, "index.html", "text/plain", bucket, timestamp, "<html></html>")
				Expect(err).ToNot(HaveOccurred())

				// Simulate manifest storage failure
				mockService.Errors["SetManifest"] = fmt.Errorf("manifest service unavailable")

				// Attempt to store manifest - should fail
				manifest := impl.Manifest{
					Files:     []string{"index.html"},
					Image:     "test-image:fail",
					Timestamp: timestamp,
				}
				err = mockService.SetManifest(namespace, bucket, timestamp, manifest)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("manifest service unavailable"))

				// Verify assets are still stored but manifest is not
				_, exists := mockService.GetStoredItem(namespace, "index.html")
				Expect(exists).To(BeTrue())
				_, exists = mockService.GetStoredManifest(namespace, timestamp)
				Expect(exists).To(BeFalse())
			})

			It("should handle cleanup failure gracefully", func() {
				namespace := "cleanup-fail-app"
				bucket := "test-bucket"

				// Store some test data
				timestamp := time.Now().Unix() - 7200 // 2 hours ago
				err := mockService.SetManifest(namespace, bucket, timestamp, impl.Manifest{
					Files:     []string{"old-file.txt"},
					Image:     "test-image:old",
					Timestamp: timestamp,
				})
				Expect(err).ToNot(HaveOccurred())

				// Simulate cleanup failure
				mockService.Errors["CleanupCache"] = fmt.Errorf("cleanup service down")

				// Attempt cleanup - should fail
				err = mockService.CleanupCache(namespace, bucket, 3600, 1)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("cleanup service down"))

				// Verify data is still there (cleanup didn't run)
				_, exists := mockService.GetStoredManifest(namespace, timestamp)
				Expect(exists).To(BeTrue())
			})
		})
	})

	Describe("Performance simulation", func() {
		Context("Large deployment", func() {
			It("should handle deployment with many files", func() {
				namespace := "large-app"
				bucket := "performance-test"
				timestamp := time.Now().Unix()

				// Simulate a large frontend app with many assets
				fileCount := 50
				files := make(map[string]string)

				// Generate test files
				for i := 0; i < fileCount; i++ {
					filepath := fmt.Sprintf("assets/file_%d.js", i)
					content := fmt.Sprintf("// File %d content\nconsole.log('File %d loaded');", i, i)
					files[filepath] = content
				}

				// Add common files
				files["index.html"] = "<html><body>Large App</body></html>"
				files["main.css"] = "body { margin: 0; }"
				files["bundle.js"] = "// Main application bundle"

				// Start deployment
				startTime := time.Now()
				err := mockService.StartPopulate(namespace, bucket, timestamp)
				Expect(err).ToNot(HaveOccurred())

				// Store all files
				for filepath, content := range files {
					err := mockService.SetItem(namespace, filepath, "text/plain", bucket, timestamp, content)
					Expect(err).ToNot(HaveOccurred())
				}

				// Create manifest
				manifestFiles := []string{}
				for filepath := range files {
					manifestFiles = append(manifestFiles, filepath)
				}
				manifest := impl.Manifest{
					Files:     manifestFiles,
					Image:     "test-image:large",
					Timestamp: timestamp,
				}
				err = mockService.SetManifest(namespace, bucket, timestamp, manifest)
				Expect(err).ToNot(HaveOccurred())

				err = mockService.EndPopulate(namespace, bucket, timestamp)
				Expect(err).ToNot(HaveOccurred())

				deploymentTime := time.Since(startTime)

				// Verify all files were stored
				for filepath, expectedContent := range files {
					storedContent, exists := mockService.GetStoredItem(namespace, filepath)
					Expect(exists).To(BeTrue())
					Expect(storedContent).To(Equal(expectedContent))
				}

				// Verify manifest contains all files
				storedManifest, exists := mockService.GetStoredManifest(namespace, timestamp)
				Expect(exists).To(BeTrue())
				Expect(len(storedManifest.Files)).To(Equal(len(files)))

				// Performance should be reasonable (mocked operations should be fast)
				Expect(deploymentTime).To(BeNumerically("<", time.Second))

				fmt.Printf("Deployed %d files in %v\n", len(files), deploymentTime)
			})
		})
	})

	Describe("Data consistency checks", func() {
		Context("File integrity", func() {
			It("should maintain file content integrity", func() {
				namespace := "integrity-test"
				bucket := "test-bucket"
				timestamp := time.Now().Unix()

				// Test various content types
				testFiles := map[string]string{
					"text.txt":          "Simple text content",
					"json.json":         `{"key": "value", "number": 42, "array": [1,2,3]}`,
					"html.html":         "<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Hello</h1></body></html>",
					"css.css":           "body { font-family: 'Arial', sans-serif; color: #333; }",
					"js.js":             "function test() { return 'Hello World'; } console.log(test());",
					"binary-like.bin":   string([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}),
					"unicode.txt":       "Hello 世界 🌍 Ñoño café résumé",
					"large-content.txt": strings.Repeat("This is a large file content. ", 1000),
					"empty.txt":         "",
					"whitespace.txt":    "   \n\t\r\n   ",
				}

				// Store all test files
				for filepath, content := range testFiles {
					err := mockService.SetItem(namespace, filepath, "text/plain", bucket, timestamp, content)
					Expect(err).ToNot(HaveOccurred())
				}

				// Verify content integrity for each file
				for filepath, originalContent := range testFiles {
					retrievedContent, exists := mockService.GetStoredItem(namespace, filepath)
					Expect(exists).To(BeTrue(), fmt.Sprintf("File %s should exist", filepath))
					Expect(retrievedContent).To(Equal(originalContent), fmt.Sprintf("Content mismatch for %s", filepath))
					Expect(len(retrievedContent)).To(Equal(len(originalContent)), fmt.Sprintf("Length mismatch for %s", filepath))
				}
			})

			It("should handle file overwrites correctly", func() {
				namespace := "overwrite-test"
				bucket := "test-bucket"
				timestamp := time.Now().Unix()
				filepath := "overwrite-me.txt"

				// Store initial content
				originalContent := "Original content"
				err := mockService.SetItem(namespace, filepath, "text/plain", bucket, timestamp, originalContent)
				Expect(err).ToNot(HaveOccurred())

				// Verify initial content
				content, exists := mockService.GetStoredItem(namespace, filepath)
				Expect(exists).To(BeTrue())
				Expect(content).To(Equal(originalContent))

				// Overwrite with new content
				newContent := "Updated content that is completely different"
				err = mockService.SetItem(namespace, filepath, "text/plain", bucket, timestamp, newContent)
				Expect(err).ToNot(HaveOccurred())

				// Verify content was overwritten
				content, exists = mockService.GetStoredItem(namespace, filepath)
				Expect(exists).To(BeTrue())
				Expect(content).To(Equal(newContent))
				Expect(content).ToNot(Equal(originalContent))
			})
		})
	})
})
