FROM registry.access.redhat.com/ubi9/go-toolset:latest AS builder
USER root

WORKDIR /opt/app-root/src/valpop
COPY . .
# statically building so it doesn't depend on GLIBC
RUN CGO_ENABLED=0 go build -o valpop -ldflags="-s -w"

FROM registry.access.redhat.com/ubi9-minimal:latest

COPY --from=builder /opt/app-root/src/valpop/valpop /usr/local/bin/valpop
USER 1001

ENTRYPOINT ["/usr/local/bin/valpop"]
