package s3_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/valpop/impl/mock"
	"github.com/RedHatInsights/valpop/impl/s3"
)

var _ = Describe("S3 Implementation with Mocks", func() {
	var (
		mockService   *mock.S3Service
		testNamespace = "testapp"
		testBucket    = "test-bucket"
		testTimestamp = int64(1234567890)
	)

	BeforeEach(func() {
		mockService = mock.NewS3Service()
	})

	Describe("File Storage Operations", func() {
		Context("SetItem", func() {
			It("should handle SetItem errors", func() {
				mockService.Errors["SetItem"] = fmt.Errorf("storage error")

				err := mockService.SetItem("ns", "file.txt", "text/plain", "bucket", 123, "content")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("storage error"))
			})
		})

		Context("SetManifest", func() {
			It("should store manifests correctly", func() {
				manifest := s3.Manifest{"index.html", "style.css", "app.js"}

				err := mockService.SetManifest(testNamespace, testBucket, testTimestamp, manifest)p, manifest)
				Expect(err).ToNot(HaveOccurred())

				// Verify the manifest was stored
				storedManifest, exists := mockService.GetStoredManifest(testNamespace, testTimestamp)
				Expect(exists).To(BeTrue())
				Expect(storedManifest).To(Equal(manifest))

				// Verify operation was tracked
				Expect(mockService.Operations).To(ContainElement("SetManifest"))
			})

			It("should handle SetManifest errors", func() {
				mockService.Errors["SetManifest"] = fmt.Errorf("manifest storage error")

				err := mockService.SetManifest("ns", "bucket", 123, s3.Manifest{"file.txt"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("manifest storage error"))
			})

			It("should store multiple manifests with different timestamps", func() {
				namespace := "app"
				manifests := map[int64]s3.Manifest{
					1000: {"file1.txt", "file2.txt"},
					2000: {"file1.txt", "file2.txt", "file3.txt"},
					3000: {"file1.txt", "file3.txt"},
				}

				// Store all manifests
				for timestamp, manifest := range manifests {
					err := mockService.SetManifest(namespace, "bucket", timestamp, manifest)
					Expect(err).ToNot(HaveOccurred())
				}

				// Verify all manifests are stored correctly
				for timestamp, expectedManifest := range manifests {
					storedManifest, exists := mockService.GetStoredManifest(namespace, timestamp)
					Expect(exists).To(BeTrue())
					Expect(storedManifest).To(Equal(expectedManifest))
				}
			})
		})
	})

	Describe("File Deletion Operations", func() {
		Context("CleanupCache", func() {
			It("should delete old manifests based on timeout", func() {
				currentTime := time.Now().Unix()

				// Create manifests with different timestamps
				oldTimestamp := currentTime - 3600   // 1 hour ago
				recentTimestamp := currentTime - 300 // 5 minutes ago

				oldManifest := s3.Manifest{"old-file1.txt", "old-file2.txt"}
				recentManifest := s3.Manifest{"recent-file1.txt", "recent-file2.txt"}

				// Store manifests
				err := mockService.SetManifest(testNamespace, testBucket, oldTimestamp, oldManifest)anifest)
				Expect(err).ToNot(HaveOccurred())
				err = mockService.SetManifest(testNamespace, testBucket, recentTimestamp, recentManifest)
				Expect(err).ToNot(HaveOccurred())

				// Verify both manifests exist
				_, exists := mockService.GetStoredManifest(testNamespace, oldTimestamp)
				Expect(exists).To(BeTrue())
				_, exists = mockService.GetStoredManifest(testNamespace, recentTimestamp)
				Expect(exists).To(BeTrue())

				// Cleanup with 30-minute timeout
				timeout := int64(1800) // 30 minutes
				err = mockService.CleanupCache(testNamespace, testBucket, timeout, 1)
				Expect(err).ToNot(HaveOccurred())

				// Verify old manifest was deleted but recent one remains
				_, exists = mockService.GetStoredManifest(testNamespace, oldTimestamp)
				Expect(exists).To(BeFalse()) // Should be deleted
				_, exists = mockService.GetStoredManifest(testNamespace, recentTimestamp)
				Expect(exists).To(BeTrue()) // Should remain
			})

			It("should handle CleanupCache errors", func() {
				mockService.Errors["CleanupCache"] = fmt.Errorf("cleanup error")

				err := mockService.CleanupCache("ns", "bucket", 1800, 1)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("cleanup error"))
			})

			It("should respect minimum asset records constraint", func() {
				timeout := int64(1) // Very short timeout - everything should be old

				// Create multiple old manifests
				timestamps := []int64{1000, 2000, 3000, 4000}
				for _, ts := range timestamps {
					manifest := s3.Manifest{fmt.Sprintf("file-%d.txt", ts)}
					err := mockService.SetManifest(testNamespace, testBucket, ts, manifest)
					Expect(err).ToNot(HaveOccurred())
				}

				// Cleanup with minimum 2 records
				err := mockService.CleanupCache(testNamespace, testBucket, timeout, 2)
				Expect(err).ToNot(HaveOccurred())

				// Count remaining manifests
				remainingCount := 0
				for _, ts := range timestamps {
					if _, exists := mockService.GetStoredManifest(testNamespace, ts); exists {
						remainingCount++
					}
				}

				// Should have at least kept minimum manifests despite timeout
				Expect(remainingCount).To(BeNumerically(">=", 2))
			})
		})

		Context("Manual deletion", func() {
			It("should allow manual file deletion", func() {
				filepath := "temp-file.txt"
				content := "temporary content"

				// Store a file
				err := mockService.SetItem(testNamespace, filepath, "text/plain", "bucket", 123, content)
				Expect(err).ToNot(HaveOccurred())

				// Verify it exists
				_, exists := mockService.GetStoredItem(testNamespace, filepath)
				Expect(exists).To(BeTrue())

				// Delete it manually
				mockService.DeleteItem(testNamespace, filepath)

				// Verify it's gone
				_, exists = mockService.GetStoredItem(testNamespace, filepath)
				Expect(exists).To(BeFalse())
			})
		})
	})

	Describe("Lifecycle Operations", func() {
		Context("Complete populate workflow", func() {
			It("should execute populate workflow operations in correct order", func() {
				// Execute populate workflow
				err := mockService.StartPopulate(testNamespace, testBucket, 123)et, 123)
				Expect(err).ToNot(HaveOccurred())

				err = mockService.SetItem(testNamespace, "index.html", "text/html", testBucket, 123, "<html></html>")/html>")
				Expect(err).ToNot(HaveOccurred())

				err = mockService.SetManifest(testNamespace, testBucket, 123, s3.Manifest{"index.html"}).html"})
				Expect(err).ToNot(HaveOccurred())

				err = mockService.EndPopulate(testNamespace, testBucket, 123)et, 123)
				Expect(err).ToNot(HaveOccurred())

				err = mockService.CleanupCache(testNamespace, testBucket, 3600, 3)
				Expect(err).ToNot(HaveOccurred())

				mockService.Close()

				// Verify all operations were called in order
				expectedOps := []string{"StartPopulate", "SetItem", "SetManifest", "EndPopulate", "CleanupCache", "Close"}
				Expect(mockService.Operations).To(Equal(expectedOps))
			})

			It("should handle populate function call", func() {
				err := mockService.PopulateFn("addr", "bucket", "source", "prefix", 3600, 3)
				Expect(err).ToNot(HaveOccurred())
				Expect(mockService.Operations).To(ContainElement("PopulateFn"))
			})
		})
	})

	Describe("Error handling", func() {
		Context("Storage errors", func() {
			It("should propagate storage errors correctly", func() {
				// Test SetItem error
				mockService.Errors["SetItem"] = fmt.Errorf("disk full")
				err := mockService.SetItem("ns", "file.txt", "text/plain", "bucket", 123, "content")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("disk full"))

				// Test SetManifest error
				mockService.Errors["SetManifest"] = fmt.Errorf("network error")
				err = mockService.SetManifest("ns", "bucket", 123, s3.Manifest{"file.txt"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("network error"))

				// Test PopulateFn error
				mockService.Errors["PopulateFn"] = fmt.Errorf("source directory not found")
				err = mockService.PopulateFn("addr", "bucket", "source", "prefix", 3600, 3)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("source directory not found"))
			})
		})
	})
})
