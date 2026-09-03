# ---- Build stage ----
FROM golang:1.27.1-alpine3.24 AS builder

ARG MAINVERSION=2.2.0

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the Go source
COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${MAINVERSION}" -o mimir-redirect .

# ---- Final stage: distroless static, no shell, no libc, no interpreter ----
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot AS base
WORKDIR /
COPY --from=builder /workspace/mimir-redirect .
USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/mimir-redirect"]
