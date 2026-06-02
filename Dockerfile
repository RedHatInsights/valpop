FROM registry.access.redhat.com/ubi9/go-toolset:latest@sha256:49f5929f6674d75377902ddcc2f46baf7a5cfcaada2497ee43f66e090943afd6 AS builder
USER root

WORKDIR /opt/app-root/src/valpop

# only copy the necessary files
COPY go.mod go.sum main.go .
COPY cmd/ cmd/
COPY impl/ impl/

# statically building so it doesn't depend on GLIBC
RUN CGO_ENABLED=0 go build -o valpop -ldflags="-s -w"

FROM registry.access.redhat.com/ubi9-minimal:latest@sha256:ae09ecc3d754bc1726cbda3e2599cc7839e09fe1cc547ce173cf669b645be3cc

RUN microdnf update -y

COPY --from=builder /opt/app-root/src/valpop/valpop /usr/local/bin/valpop
USER 1001

ENTRYPOINT ["/usr/local/bin/valpop"]
