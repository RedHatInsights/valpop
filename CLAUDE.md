@AGENTS.md

## Development Commands

### Build
```bash
make build              # go build
```

### Run locally
```bash
make start-minio        # Start MinIO S3 server
make create-minio-buckets  # Create required buckets
make start-valkey       # Start Valkey server
make start-valkey-cli   # Interactive Valkey CLI
```

### Test uploads
```bash
make minio-test-upload  # Upload test files to MinIO
make valkey-test-upload # Upload test files to Valkey
```

### Testing
```bash
make test               # go test ./...
make test-verbose       # Ginkgo verbose output
make test-succinct      # Ginkgo succinct output
```

Never call `ginkgo` directly -- use `go run github.com/onsi/ginkgo/v2/ginkgo` or `make test`.

### Container
```bash
podman build -f Dockerfile -t valpop .
podman run -it valpop {args}
```

## Git Conventions

- Branch naming: `type/short-description` (e.g., `fix/cleanup-logic`, `feat/add-pop-s3`)
- Conventional commits: `type(scope): description`
- Scopes: `s3`, `valkey`, `cli`, `ci`, `docs`

## Key Files

- `cmd/root.go` -- global CLI flags and validation
- `cmd/populate.go` -- populate command with storage mode routing
- `cmd/pop.go` -- pop command (Valkey only currently)
- `impl/impl.go` -- shared types, business logic, manifest parsing
- `impl/s3/s3.go` -- S3/MinIO implementation
- `impl/s3/interface.go` -- S3 client and service interfaces
- `impl/valkey/valkey.go` -- Valkey implementation
- `impl/mock/s3.go` -- mock S3 service for testing
- `Dockerfile` -- multi-stage UBI 9 build
- `.tekton/` -- Konflux CI/CD pipelines
