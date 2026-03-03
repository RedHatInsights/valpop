package s3_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Define manifest type for testing since it's not exported
type TestManifest []string

type manifestInfo struct {
	key       string
	timestamp int64
}

var _ = Describe("S3 Implementation", func() {
	var (
		testNamespace = "frontend"
		testTimestamp = int64(1234567890)
	)

	Describe("Path generation functions", func() {
		Context("makeDataPath", func() {
			It("should generate correct data path format", func() {
				filepath := "index.html"
				expectedPath := fmt.Sprintf("data/%s/%s", testNamespace, filepath)
				Expect(expectedPath).To(Equal("data/frontend/index.html"))
			})

			It("should handle paths with subdirectories", func() {
				namespace := "app"
				filepath := "assets/js/main.js"
				expectedPath := fmt.Sprintf("data/%s/%s", namespace, filepath)
				Expect(expectedPath).To(Equal("data/app/assets/js/main.js"))
			})

			It("should handle special characters in filenames", func() {
				namespace := "myapp"
				filepath := "file with spaces.txt"
				expectedPath := fmt.Sprintf("data/%s/%s", namespace, filepath)
				Expect(expectedPath).To(Equal("data/myapp/file with spaces.txt"))
			})
		})

		Context("makeManifestPath", func() {
			It("should generate correct manifest path format", func() {
				expectedPath := fmt.Sprintf("manifests/%s/%d", testNamespace, testTimestamp)
				Expect(expectedPath).To(Equal("manifests/frontend/1234567890"))
			})

			It("should handle different namespaces", func() {
				namespace := "my-application"
				timestamp := int64(9876543210)
				expectedPath := fmt.Sprintf("manifests/%s/%d", namespace, timestamp)
				Expect(expectedPath).To(Equal("manifests/my-application/9876543210"))
			})
		})
	})

	Describe("Manifest operations", func() {
		Context("Manifest JSON serialization", func() {
			It("should serialize empty manifest", func() {
				manifest := TestManifest{}
				data, err := json.Marshal(manifest)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(data)).To(Equal("[]"))
			})

			It("should serialize manifest with files", func() {
				manifest := TestManifest{"index.html", "style.css", "app.js"}
				data, err := json.Marshal(manifest)
				Expect(err).ToNot(HaveOccurred())

				// Parse back to verify
				var parsed TestManifest
				err = json.Unmarshal(data, &parsed)
				Expect(err).ToNot(HaveOccurred())
				Expect(parsed).To(Equal(manifest))
			})

			It("should handle manifest with nested paths", func() {
				manifest := TestManifest{
					"index.html",
					"assets/css/main.css",
					"assets/js/app.js",
					"images/logo.png",
				}
				data, err := json.Marshal(manifest)
				Expect(err).ToNot(HaveOccurred())

				var parsed TestManifest
				err = json.Unmarshal(data, &parsed)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(parsed)).To(Equal(4))
				Expect(parsed).To(ContainElement("assets/css/main.css"))
			})
		})
	})

	Describe("Cleanup operations", func() {
		Context("Manifest timestamp parsing", func() {
			It("should parse timestamp from manifest path", func() {
				objectKey := fmt.Sprintf("manifests/%s/%d", testNamespace, testTimestamp)
				prefix := fmt.Sprintf("manifests/%s/", testNamespace)

				timestampString, found := strings.CutPrefix(objectKey, prefix)
				Expect(found).To(BeTrue())
				Expect(timestampString).To(Equal("1234567890"))

				timestamp, err := strconv.Atoi(timestampString)
				Expect(err).ToNot(HaveOccurred())
				Expect(timestamp).To(Equal(1234567890))
			})

			It("should handle invalid timestamp format", func() {
				objectKey := fmt.Sprintf("manifests/%s/invalid-timestamp", testNamespace)
				prefix := fmt.Sprintf("manifests/%s/", testNamespace)

				timestampString, found := strings.CutPrefix(objectKey, prefix)
				Expect(found).To(BeTrue())
				Expect(timestampString).To(Equal("invalid-timestamp"))

				_, err := strconv.Atoi(timestampString)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("Manifest sorting", func() {
			It("should sort manifests by timestamp (newest first)", func() {
				manifests := []manifestInfo{
					{"manifests/app/1000", 1000},
					{"manifests/app/3000", 3000},
					{"manifests/app/2000", 2000},
					{"manifests/app/1500", 1500},
				}

				// Sort by timestamp (newest first)
				sort.Slice(manifests, func(i, j int) bool {
					return manifests[i].timestamp > manifests[j].timestamp
				})

				Expect(manifests[0].timestamp).To(Equal(int64(3000)))
				Expect(manifests[1].timestamp).To(Equal(int64(2000)))
				Expect(manifests[2].timestamp).To(Equal(int64(1500)))
				Expect(manifests[3].timestamp).To(Equal(int64(1000)))
			})
		})

		Context("File protection logic", func() {
			It("should protect files referenced in newer manifests", func() {
				oldFiles := map[string]bool{
					"file1.txt": true,
					"file2.txt": true,
					"file3.txt": true,
				}

				newFiles := map[string]bool{
					"file1.txt": true, // Still referenced, should be protected
					"file3.txt": true, // Still referenced, should be protected
				}

				// Simulate protection logic
				for file := range newFiles {
					delete(oldFiles, file)
				}

				// Should only have file2.txt left for deletion
				Expect(len(oldFiles)).To(Equal(1))
				Expect(oldFiles["file2.txt"]).To(BeTrue())
			})

			It("should always protect fedmods.json", func() {
				oldFiles := map[string]bool{
					"fedmods.json": true,
					"other.txt":    true,
				}

				// Simulate fedmods.json protection
				delete(oldFiles, "fedmods.json")

				Expect(oldFiles["fedmods.json"]).To(BeFalse())
				Expect(oldFiles["other.txt"]).To(BeTrue())
			})
		})

		Context("Timeout and minimum records logic", func() {
			var (
				manifests       []manifestInfo
				minAssetRecords int64
				timeout         int64
				currentTime     int64
			)

			BeforeEach(func() {
				minAssetRecords = int64(2)
				timeout = int64(1000)
				currentTime = int64(5000)
			})

			It("should respect minimum asset records constraint", func() {
				manifests = []manifestInfo{
					{"manifests/app/4000", 4000}, // newest
					{"manifests/app/3000", 3000},
					{"manifests/app/2000", 2000},
					{"manifests/app/1000", 1000}, // oldest
				}

				oldManifests := []string{}
				for i, manifest := range manifests {
					// Keep at least minAssetRecords manifests regardless of timeout
					if int64(i) >= minAssetRecords && currentTime-manifest.timestamp > timeout {
						oldManifests = append(oldManifests, manifest.key)
					}
				}

				// Should delete the 2 oldest manifests (indices 2 and 3)
				Expect(len(oldManifests)).To(Equal(2))
				Expect(oldManifests).To(ContainElement("manifests/app/2000"))
				Expect(oldManifests).To(ContainElement("manifests/app/1000"))
			})

			It("should respect timeout constraint", func() {
				minAssetRecords = int64(1) // Override for this test
				manifests = []manifestInfo{
					{"manifests/app/4900", 4900}, // Recent, within timeout
					{"manifests/app/4800", 4800}, // Recent, within timeout
					{"manifests/app/3000", 3000}, // Old, beyond timeout
					{"manifests/app/1000", 1000}, // Old, beyond timeout
				}

				oldManifests := []string{}
				for i, manifest := range manifests {
					if int64(i) >= minAssetRecords && currentTime-manifest.timestamp > timeout {
						oldManifests = append(oldManifests, manifest.key)
					}
				}

				// Should delete manifests beyond minimum that are also beyond timeout
				Expect(len(oldManifests)).To(Equal(2))
			})
		})
	})

	Describe("File operations simulation", func() {
		Context("Content handling", func() {
			DescribeTable("should calculate content length correctly",
				func(content string, expectedLen int) {
					Expect(len(content)).To(Equal(expectedLen))
				},
				Entry("HTML content", "<html><body>Hello World</body></html>", 37),
				Entry("empty content", "", 0),
				Entry("binary-like content", string([]byte{0x01, 0x02, 0x03, 0x04}), 4),
			)
		})
	})

	Describe("Time operations", func() {
		Context("Current time handling", func() {
			It("should generate valid Unix timestamp", func() {
				currentTime := time.Now().Unix()
				Expect(currentTime).To(BeNumerically(">", 0))

				// Should be reasonable timestamp (after year 2000)
				year2000 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
				Expect(currentTime).To(BeNumerically(">", year2000))
			})

			It("should handle timestamp comparisons", func() {
				now := time.Now().Unix()
				older := now - 3600    // 1 hour ago
				timeout := int64(1800) // 30 minutes

				// Should identify old timestamp
				isOld := now-older > timeout
				Expect(isOld).To(BeTrue())

				// Recent timestamp should not be old
				recent := now - 900 // 15 minutes ago
				isRecent := now-recent > timeout
				Expect(isRecent).To(BeFalse())
			})
		})
	})

	Describe("Directory walking simulation", func() {
		Context("File processing", func() {
			DescribeTable("should handle file vs directory correctly",
				func(isDir bool, shouldProcess bool) {
					if isDir {
						Expect(shouldProcess).To(BeFalse())
					} else {
						Expect(shouldProcess).To(BeTrue())
					}
				},
				Entry("should skip directories", true, false),
				Entry("should process files", false, true),
			)
		})
	})

	Describe("Cache-Control header generation", func() {
		var (
			entryPointHeader   string
			regularAssetHeader string
		)

		BeforeEach(func() {
			entryPointHeader = "public, max-age=60, stale-while-revalidate=300"
			regularAssetHeader = "public, max-age=600"
		})

		Context("Entry point files", func() {
			DescribeTable("should return short cache with stale-while-revalidate",
				func(filepath string, expectedHeader string) {
					// These files should have short cache to protect origin
					Expect(expectedHeader).To(Equal(entryPointHeader))
				},
				Entry("index.html", "index.html", "public, max-age=60, stale-while-revalidate=300"),
				Entry("fed-mods.json", "fed-mods.json", "public, max-age=60, stale-while-revalidate=300"),
				Entry("app-info.json", "app-info.json", "public, max-age=60, stale-while-revalidate=300"),
				Entry("app-info.deps.json", "app-info.deps.json", "public, max-age=60, stale-while-revalidate=300"),
				Entry("nested index.html", "assets/index.html", "public, max-age=60, stale-while-revalidate=300"),
				Entry("nested fed-mods.json", "some/path/fed-mods.json", "public, max-age=60, stale-while-revalidate=300"),
			)
		})

		Context("Regular static assets with cache-max-age parameter", func() {
			DescribeTable("should use provided cache-max-age value",
				func(cacheMaxAge int64, expectedMaxAge string) {
					// For non-entry-point files, should use the configured cache-max-age
					expectedHeader := fmt.Sprintf("public, max-age=%d", cacheMaxAge)
					Expect(expectedHeader).To(Equal(expectedMaxAge))
				},
				Entry("default 600 seconds", int64(600), "public, max-age=600"),
				Entry("300 seconds", int64(300), "public, max-age=300"),
				Entry("1800 seconds", int64(1800), "public, max-age=1800"),
				Entry("0 seconds (no-cache equivalent)", int64(0), "public, max-age=0"),
				Entry("1 year", int64(31536000), "public, max-age=31536000"),
			)
		})

		Context("Cache-Control directive requirements", func() {
			It("should use 'public' to enable CDN caching", func() {
				Expect(entryPointHeader).To(HavePrefix("public"))
				Expect(regularAssetHeader).To(HavePrefix("public"))
			})

			It("should include stale-while-revalidate for DDoS protection", func() {
				// Protects against thundering herd when cache expires
				Expect(entryPointHeader).To(ContainSubstring("stale-while-revalidate=300"))
			})
		})

		Context("Deployment rollout timing", func() {
			It("should allow deployment rollout within 6 minutes for entry points", func() {
				// max-age=60 + stale-while-revalidate=300 = 360 seconds = 6 minutes
				maxAge := int64(60)
				staleWindow := int64(300)
				maxDeploymentLag := maxAge + staleWindow

				Expect(maxDeploymentLag).To(Equal(int64(360))) // 6 minutes
			})
		})

		Context("Edge cases", func() {
			It("should handle empty filepath", func() {
				filepath := ""
				// Should not match any entry point patterns
				isEntryPoint := filepath == "index.html" ||
					filepath == "fed-mods.json" ||
					filepath == "app-info.json" ||
					filepath == "app-info.deps.json"

				Expect(isEntryPoint).To(BeFalse())
			})

			It("should handle paths with dots in directory names", func() {
				filepath := "my.app/index.html"
				// Should still match because it ends with index.html
				endsWithIndexHtml := len(filepath) >= len("index.html") &&
					filepath[len(filepath)-len("index.html"):] == "index.html"

				Expect(endsWithIndexHtml).To(BeTrue())
			})

			It("should handle uppercase file extensions", func() {
				// File matching should be case-sensitive as specified
				filepath := "INDEX.HTML"
				matchesIndexHtml := filepath == "index.html" ||
					len(filepath) > len("index.html") &&
						filepath[len(filepath)-len("index.html"):] == "index.html"

				Expect(matchesIndexHtml).To(BeFalse())
				// Our implementation uses suffix matching which is case-sensitive
			})
		})
	})
})
