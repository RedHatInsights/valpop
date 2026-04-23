# Architecture Guidelines

## System Overview

Valpop is a CLI tool that bridges frontend build containers and storage backends. It runs as a sidecar or init container alongside frontend deployments to populate a shared cache with static assets and later retrieve them for serving.

```
Frontend Container                  Storage Backend
+------------------+               +------------------+
|  Build artifacts |  -- populate ->|  S3 / Valkey     |
|  /dist/          |               |                  |
+------------------+               +------------------+
                                          |
Serving Container                         |
+------------------+               <-- pop --
|  /var/www/html/  |
+------------------+
```

## Storage Abstraction

### Interface Hierarchy

```
impl.Implementation (core interface)
  |-- StartPopulate, EndPopulate
  |-- SetItem, GetItem, DelKeys
  |-- PopulateFromDir, Pop
  |
  +-- s3.S3Service (extends with S3-specific ops)
  |     |-- SetManifest
  |     |-- PopulateFn
  |     |-- CleanupCache
  |     +-- impl: s3.Minio (uses s3.S3Client)
  |
  +-- valkey.Valkey (direct implementation)
        |-- PopulateFn
        +-- PopFn (pop is Valkey-only)
```

### S3Client Abstraction

The `S3Client` interface wraps `minio-go` methods for testability:

```go
type S3Client interface {
    PutObject(ctx, bucket, key string, reader io.Reader, size int64, opts) (UploadInfo, error)
    GetObject(ctx, bucket, key string, opts) (*Object, error)
    RemoveObject(ctx, bucket, key string, opts) error
    ListObjects(ctx, bucket string, opts) <-chan ObjectInfo
}
```

Production code uses `minio.Client` (which satisfies this interface). Tests use `MockS3Service`.

### Adding a New Storage Backend

1. Create `impl/<backend>/<backend>.go`
2. Implement `impl.Implementation` interface
3. Add mode check in `cmd/populate.go` (`viper.GetString("mode")` switch)
4. Add mode check in `cmd/pop.go` if pop is supported
5. Add flag validation in `cmd/root.go` `PersistentPreRunE` if needed
6. Add docker-compose service for local testing

## Configuration

### Flag/Env Var Binding

All CLI flags are dual-bound via cobra + viper:

```go
// In init():
rootCmd.PersistentFlags().StringP("hostname", "a", "127.0.0.1", "Storage hostname")
viper.BindPFlag("hostname", rootCmd.PersistentFlags().Lookup("hostname"))

// Access anywhere:
viper.GetString("hostname")  // reads flag or VALPOP_HOSTNAME env var
```

Priority: CLI flag > env var > default value.

### Global vs Subcommand Flags

| Scope | Flags | Defined In |
|-------|-------|-----------|
| Global (all commands) | `hostname`, `port`, `mode`, `username`, `password`, `bucket` | `cmd/root.go` |
| `populate` only | `source`, `prefix`, `image`, `valpop-image`, `timeout`, `min-asset-records`, `cache-max-age` | `cmd/populate.go` |
| `pop` only | `dest` | `cmd/pop.go` |

## Shared Business Logic

Core logic lives in `impl/impl.go`, independent of storage backend:

| Function | Purpose |
|----------|---------|
| `MakeDataKey(namespace, filepath)` | Generate consistent data key format |
| `MakeManifestKey(namespace, timestamp)` | Generate consistent manifest key format |
| `GetContentType(filepath)` | Map file extension to MIME type |
| `DetermineManifestsToDelete(manifests, time, timeout, minRecords)` | Retention policy: which manifests to remove |
| `DetermineFilesToDelete(old, kept, protected)` | Which files to remove (not referenced by kept manifests) |
| `SeparateManifests(manifests, time, timeout, minRecords)` | Split into delete vs keep lists |
| `BuildPopulateManifest(fs, callback)` | Walk filesystem, collect files via callback |
| `ParseManifest(rawData)` | Parse manifest JSON (supports old array + new object format) |

When adding new logic, prefer adding to `impl/impl.go` if it's storage-agnostic.

## Container Build

```dockerfile
# Stage 1: Build with UBI 9 go-toolset
FROM registry.access.redhat.com/ubi9/go-toolset:latest AS builder
RUN CGO_ENABLED=0 go build -o valpop -ldflags="-s -w"

# Stage 2: Minimal runtime with UBI 9
FROM registry.access.redhat.com/ubi9-minimal:latest
COPY --from=builder /opt/app-root/src/valpop/valpop /usr/local/bin/valpop
ENTRYPOINT ["/usr/local/bin/valpop"]
```

Key design decisions:
- **Static binary** (`CGO_ENABLED=0`): no glibc dependency, works on minimal images
- **Stripped symbols** (`-ldflags="-s -w"`): smaller binary size
- **Non-root user** (`USER 1001`): security best practice
- **Minimal base image**: `ubi9-minimal` (~30MB) instead of full UBI 9

## CI/CD (Tekton/Konflux)

Two pipelines in `.tekton/`:

| Pipeline | Trigger | Purpose |
|----------|---------|---------|
| `valpop-pull-request.yaml` | PR to `main` | Build image + run unit tests |
| `valpop-push.yaml` | Push to `main` | Build + push release image |

CI runs `run-unit-tests.sh` which executes Ginkgo tests. The pipeline also builds the Docker image to verify it compiles cleanly.

Test environment variables are passed via `env-vars` param in the pipeline spec.
