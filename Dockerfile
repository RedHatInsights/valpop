FROM registry.access.redhat.com/ubi9/go-toolset:latest@sha256:a2ba4645e7c424b08aa83ed7792e279683b0d33acbc5131b18183fd21e336c55 AS builder
USER root

WORKDIR /opt/app-root/src/valpop

# only copy the necessary files
COPY go.mod go.sum main.go .
COPY cmd/ cmd/
COPY impl/ impl/

# statically building so it doesn't depend on GLIBC
RUN CGO_ENABLED=0 go build -o valpop -ldflags="-s -w"

FROM registry.access.redhat.com/ubi9-minimal:latest@sha256:5b74fce9d6e629942a0c6dc0f546c193e70d7f974d999a48c948c53dd3d36362

RUN microdnf update -y

COPY --from=builder /opt/app-root/src/valpop/valpop /usr/local/bin/valpop
USER 1001

ENTRYPOINT ["/usr/local/bin/valpop"]
