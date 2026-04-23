# Testing Guidelines

## Framework

- **Ginkgo v2** -- BDD-style test framework (`Describe`/`Context`/`It` blocks)
- **Gomega** -- assertion/matcher library (`Expect(...).To(...)`)
- Standard `go test ./...` also works (Ginkgo integrates with Go's test runner)
- CI runs tests via `run-unit-tests.sh` which uses `go run github.com/onsi/ginkgo/v2/ginkgo -r --succinct`

## Test Structure

```
cmd/
  populate_test.go           # Flag validation, required args, min-asset-records constraints
  flags_conflict_test.go     # S3 credential validation tests
impl/
  impl_test.go               # Core types: AllItems, Items, ManifestInfo operations
  s3/
    s3_suite_test.go          # Ginkgo suite bootstrap (RunSpecs)
    s3_test.go                # S3 logic tests: paths, manifests, cleanup, cache-control
    s3_mock_test.go           # Mock-based tests: full workflows, error handling, lifecycle
    s3_integration_test.go    # Integration tests (requires running MinIO)
  mock/
    s3.go                     # MockS3Service implementation
    s3_test.go                # Tests for the mock service itself
main_test.go                  # Root command tests
```

## Test Categories

### Unit Tests (no external deps)

Tests in `s3_test.go` and `impl_test.go` test pure logic without any I/O:

```go
var _ = Describe("S3 Implementation", func() {
    Describe("Path generation functions", func() {
        Context("makeDataPath", func() {
            It("should generate correct data path format", func() {
                expectedPath := fmt.Sprintf("data/%s/%s", "frontend", "index.html")
                Expect(expectedPath).To(Equal("data/frontend/index.html"))
            })
        })
    })
})
```

Key patterns:
- `DescribeTable` for parameterized tests (content types, cache-control headers)
- `BeforeEach` for shared setup (timestamps, timeout values)
- Direct logic testing without mocks (string formatting, sorting, filtering)

### Mock-Based Tests

Tests in `s3_mock_test.go` use `mock.S3Service` for full workflow testing:

```go
var _ = Describe("S3 Implementation with Mocks", func() {
    var mockService *mock.S3Service

    BeforeEach(func() {
        mockService = mock.NewS3Service()
    })

    It("should execute populate workflow in correct order", func() {
        mockService.StartPopulate(ns, bucket, 123)
        mockService.SetItem(ns, "index.html", "text/html", bucket, 123, "<html></html>")
        mockService.SetManifest(ns, bucket, 123, impl.Manifest{...})
        mockService.EndPopulate(ns, bucket, 123)
        mockService.CleanupCache(ns, bucket, 3600, 3)
        mockService.Close()

        expectedOps := []string{"StartPopulate", "SetItem", "SetManifest", "EndPopulate", "CleanupCache", "Close"}
        Expect(mockService.Operations).To(Equal(expectedOps))
    })
})
```

The `MockS3Service` provides:
- In-memory file and manifest storage (`GetStoredItem`, `GetStoredManifest`)
- Operation tracking (`Operations` slice records call order)
- Error injection (`Errors` map to simulate failures)
- Manual deletion (`DeleteItem` for cleanup testing)

### Integration Tests

`s3_integration_test.go` and `test-populate.sh` require running services:

```bash
# Start MinIO first
make start-minio
make create-minio-buckets

# Run integration tests
go test ./impl/s3/ -run Integration -v
```

Environment variables for integration tests:
- `TEST_BUCKET` -- S3 bucket name (default: `test-bucket`)
- `TEST_NAMESPACE` -- cache namespace (default: `test-populate`)
- `MINIO_ENDPOINT` -- MinIO address (default: `localhost:9000`)
- `TEST_DIR` -- source directory for test files (default: `build`)

### Command Tests

Tests in `cmd/` validate CLI flag parsing and validation:
- Required flag errors (`source`, `prefix`)
- S3 credential validation (`username`/`password` required in S3 mode)
- `min-asset-records` constraints (non-negative)
- Environment variable binding (`VALPOP_*` prefix)

## Writing New Tests

1. **New business logic**: Add to `impl/impl_test.go` if storage-agnostic, or `s3_test.go` if S3-specific
2. **New S3 operations**: Use `MockS3Service` in `s3_mock_test.go` -- inject errors via `Errors` map, verify operations via `Operations` slice
3. **New CLI flags**: Add validation tests in `cmd/populate_test.go` or `cmd/flags_conflict_test.go`
4. **New content types**: Add entries to `DescribeTable` in the content-type tests

## Checklist

- [ ] Tests use Ginkgo `Describe`/`Context`/`It` blocks (not raw `testing.T`)
- [ ] Mock-based tests create fresh `mock.NewS3Service()` in `BeforeEach`
- [ ] Error injection tests verify error message content, not just `HaveOccurred()`
- [ ] Integration tests are clearly separated and require running services
- [ ] `DescribeTable` used for parameterized test cases
- [ ] Tests pass locally: `make test`
