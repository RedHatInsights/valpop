FROM registry.access.redhat.com/hi/go:latest-fips-builder AS builder
USER 0

WORKDIR /workspace

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY main.go .
COPY cmd/ cmd/
COPY impl/ impl/

# Build with CGO enabled for FIPS-compliant crypto (BoringSSL)
RUN CGO_ENABLED=1 go build -o valpop -ldflags="-s -w"

FROM registry.access.redhat.com/hi/go:latest-fips

COPY --from=builder /workspace/valpop /usr/local/bin/valpop
USER 1001

CMD ["/usr/local/bin/valpop"]
