# Valpop -- Agent Guide

## Project Overview

Valpop is a cache populator and popper for the ConsoleDot Frontend system. It runs inside frontend containers to upload static assets (HTML, CSS, JS, images) to S3-compatible storage (MinIO) or Valkey/Redis, and retrieves them for serving. It handles cache cleanup with configurable timeout and minimum-record retention policies.

### Commands

| Command | Purpose |
|---------|---------|
| `populate` | Walk a source directory, upload all files to storage, write a manifest, then clean up old versions |
| `pop` | Pull cached files from storage into a local destination directory for serving (Valkey mode only) |

## Tech Stack

- **Go** 1.25.0 (`.go` version in `go.mod`)
- **cobra** + **viper** -- CLI framework with flag parsing and env var binding (`VALPOP_` prefix)
- **minio-go/v7** -- S3-compatible object storage client
- **valkey-go** -- Valkey/Redis client
- **Ginkgo v2** + **Gomega** -- BDD-style testing framework
- **Podman Compose** -- local dev services (Valkey 8.1.3, MinIO)
- **Tekton** -- CI/CD pipelines (Konflux)
- **UBI 9** -- base container images (go-toolset builder + ubi9-minimal runtime)

## Project Structure

```
valpop/
  main.go                          # Entrypoint: cmd.Execute()
  cmd/
    root.go                        # Root cobra command, global flags (hostname, port, mode, credentials, bucket)
    populate.go                    # Populate subcommand: walks source dir, uploads to S3 or Valkey
    pop.go                         # Pop subcommand: pulls from Valkey cache to dest dir
    populate_test.go               # Flag validation tests
    flags_conflict_test.go         # Flag conflict tests
  impl/
    impl.go                        # Core types (AllItems, Items, ManifestInfo, Manifest), shared business logic
    impl_test.go                   # Tests for core types and logic
    s3/
      interface.go                 # S3Client and S3Service interfaces (abstractions for testing)
      s3.go                        # MinIO/S3 implementation: populate, cleanup, manifest management
      s3_test.go                   # Ginkgo BDD tests for S3 logic
      s3_suite_test.go             # Ginkgo test suite bootstrap
      s3_mock_test.go              # Tests using mock S3 service
      s3_integration_test.go       # Integration tests (requires running MinIO)
    mock/
      s3.go                        # MockS3Service: in-memory S3 mock for testing
      s3_test.go                   # Tests for the mock itself
    valkey/
      valkey.go                    # Valkey implementation: populate, pop, cleanup
  Dockerfile                       # Multi-stage: UBI 9 go-toolset build -> ubi9-minimal runtime
  docker-compose.yml               # Valkey + MinIO + MinIO bucket creation services
  Makefile                         # Build, test, local dev commands
  run-unit-tests.sh                # CI unit test runner (Ginkgo)
  test-populate.sh                 # Integration test script
  .tekton/
    valpop-pull-request.yaml       # Konflux PR pipeline
    valpop-push.yaml               # Konflux push pipeline
  TESTING.md                       # Testing documentation
  README.md                        # Usage documentation
```

## Architecture

### Storage Modes

Valpop supports two storage backends, selected via `--mode` / `VALPOP_MODE`:

| Mode | Backend | Client | Use Case |
|------|---------|--------|----------|
| `s3` (default) | MinIO / S3-compatible | `minio-go/v7` | Production: S3 bucket storage |
| `valkey` | Valkey / Redis | `valkey-go` | Alternative: in-memory key-value store |

### Key Structure

**S3 mode:**
- Data: `data/{namespace}/{filepath}` (e.g., `data/myapp/index.html`)
- Manifests: `manifests/{namespace}/{timestamp}` (e.g., `manifests/myapp/1742472000`)

**Valkey mode:**
- Data: `data:{namespace}:{timestamp}:{filepath}`
- Locks: `lock:{namespace}:{timestamp}` (in-progress markers)

### Populate Flow

1. Check if latest manifest already has the same image (duplicate prevention)
2. Walk source directory, upload each file with content-type detection
3. Write manifest JSON with file list, image tag, valpop image, and timestamp
4. Clean up old versions based on retention policy

### Cleanup Logic

Two parameters control retention:
- **timeout** (`--timeout`): how old an asset must be before eligible for deletion
- **min-asset-records** (`--min-asset-records`): minimum versions always kept regardless of age

Cleanup process:
1. Sort manifests by timestamp (newest first)
2. Keep at least `min-asset-records` manifests
3. Delete manifests older than `timeout` beyond the minimum count
4. Delete files only referenced by deleted manifests (not in any kept manifest)
5. Always protect `fed-mods.json`

### Cache-Control Headers (S3 mode)

| File Pattern | Cache-Control |
|-------------|---------------|
| `index.html`, `fed-mods.json`, `app-info.json`, `app-info.deps.json` | `public, max-age=60, stale-while-revalidate=300` |
| All other assets | `public, max-age={cache-max-age}` (default: 86400) |

Entry point files use short cache (60s) with stale-while-revalidate (300s) for CDN protection during deployments.

### Manifest Format

```json
{
  "files": ["index.html", "app.js", "style.css"],
  "image": "myapp:v1.2.3",
  "valpopImage": "valpop:latest",
  "timestamp": 1742472000
}
```

Supports both new (object) and legacy (array-only) manifest formats via `ParseManifest()`.

### Interface Abstractions

- **`impl.Implementation`**: Core storage interface (`StartPopulate`, `EndPopulate`, `SetItem`, `GetItem`, `DelKeys`, `PopulateFromDir`, `Pop`)
- **`s3.S3Client`**: Abstracts MinIO client for testing (`PutObject`, `GetObject`, `RemoveObject`, `ListObjects`)
- **`s3.S3Service`**: Extends `Implementation` with S3-specific operations (`SetManifest`, `PopulateFn`, `CleanupCache`)

## Coding Conventions

1. **CLI via cobra/viper**: All flags defined in `init()` functions, bound to viper for env var support (`VALPOP_` prefix)
2. **Two storage backends**: S3 and Valkey share core types from `impl/` but have independent implementations
3. **Interface-based testing**: `S3Client` interface abstracts the MinIO client; `MockS3Service` provides in-memory implementation
4. **Business logic in `impl/`**: Shared functions (`DetermineManifestsToDelete`, `DetermineFilesToDelete`, `SeparateManifests`, `BuildPopulateManifest`, `ParseManifest`) are storage-agnostic
5. **Static binary**: Built with `CGO_ENABLED=0` for minimal container image (no glibc dependency)
6. **Content-type detection**: `GetContentType()` maps file extensions to MIME types (HTML, CSS, JS, fonts, images, JSON)
7. **Ginkgo BDD tests**: Use `Describe`/`Context`/`It` blocks with Gomega matchers; `DescribeTable` for parameterized tests
8. **Env vars with `VALPOP_` prefix**: All CLI flags also settable via environment variables

## Common Pitfalls

1. **S3 credentials required in S3 mode**: `username` and `password` are validated in `PersistentPreRunE` -- missing credentials cause immediate error
2. **Valkey pop uses oldest timestamp**: `PopFn` sorts stamps and uses `stamps[0]` (oldest), not newest. This is intentional for ordered application
3. **Integration tests need running services**: `s3_integration_test.go` and `test-populate.sh` require MinIO/Valkey containers. Unit tests use mocks
4. **Manifest format migration**: `ParseManifest` handles both old `[]string` format and new `Manifest` struct. Always write new format
5. **fed-mods.json is always protected**: Never deleted during cleanup regardless of age or version count
6. **Duplicate upload prevention**: If latest manifest has same image tag, populate is skipped entirely
