# Testing

This project uses [Ginkgo](https://onsi.github.io/ginkgo/) for BDD-style testing with [Gomega](https://onsi.github.io/gomega/) for assertions.

## Test Structure

The tests are organized as follows:

- `cmd/` - Tests for command-line interface and flag validation
- `impl/` - Tests for basic data types and storage interfaces  
- `impl/s3/` - Tests for S3/MinIO implementation including manifest handling and cleanup logic

## Running Tests

### Standard Go Tests
```bash
go test ./...
```

### Ginkgo BDD Tests (Detailed Output)
```bash
# Using local ginkgo
go run github.com/onsi/ginkgo/v2/ginkgo -r

# Succinct output
go run github.com/onsi/ginkgo/v2/ginkgo -r --succinct

# Verbose output
go run github.com/onsi/ginkgo/v2/ginkgo -r -v
```

### Using Makefile
```bash
make test
```

## Test Coverage

The tests include **90 test specifications** covering:

### Command Package (`cmd/`)
- Root command configuration and validation
- Environment variable binding with VALPOP prefix
- S3 credential validation
- Pop command (disabled, placeholder test)
- Populate command validation including timeout and min-asset-records

### Implementation Package (`impl/`)
- AllItems and Items type operations
- Data structure manipulation and nested operations
- Storage backend interface compliance

### S3 Implementation (`impl/s3/`)
- **Path generation** for data and manifests
- **JSON serialization/deserialization** of manifests
- **Cleanup operations** with timestamp parsing and sorting
- **File protection logic** for newer manifests
- **Timeout and minimum records** constraints
- **Content handling** and time operations
- **Mock implementations** for comprehensive testing without external dependencies
- **Real-world deployment scenarios** including multi-version deployments
- **Error resilience testing** including storage failures and recovery
- **Performance simulation** with large file deployments
- **Data integrity checks** for various content types and file operations

## Testing Features

### Mock Implementations
The S3 tests include comprehensive mock implementations that simulate:
- **File storage and retrieval** operations
- **Manifest management** with version tracking
- **Cleanup logic** respecting timeout and minimum record constraints
- **Error conditions** and failure scenarios
- **Performance characteristics** for large deployments

### Real-world Scenarios
The tests cover realistic application deployment scenarios:
- **Frontend app deployment** with HTML, CSS, JS, and assets
- **Multi-version deployments** with proper cleanup of old versions
- **Error resilience** during partial failures
- **Large-scale deployments** with many files
- **Content integrity** across different file types and sizes

## Testing Philosophy

The tests focus on:
- **Logic Testing**: Testing business logic without external dependencies
- **Data Structure Validation**: Ensuring correct manipulation of internal data types
- **Edge Case Handling**: Testing boundary conditions and error scenarios
- **Integration Points**: Testing interfaces between components

Since the actual S3 client requires external services, the tests simulate the core logic and data transformations rather than making actual network calls. This approach provides fast, reliable tests that can run in any environment.

### Advanced S3 Mock Testing

The S3 implementation includes sophisticated mock services that allow testing:
- **Complete deployment workflows** from start to finish
- **File storage and retrieval** with content integrity verification  
- **Multi-version deployments** with automatic cleanup of old versions
- **Error handling** for storage failures, network issues, and partial deployments
- **Performance characteristics** with large file counts and various content types
- **Edge cases** like empty files, binary content, Unicode text, and file overwrites

The mock implementation provides a `MockS3Service` that implements the same interface as the real S3 service but stores data in memory, allowing for:
- **Fast test execution** (no network I/O)
- **Deterministic behavior** (no external dependencies)
- **Error injection** for testing failure scenarios
- **Operation tracking** to verify correct call sequences
- **Data integrity verification** to ensure content is stored and retrieved correctly
