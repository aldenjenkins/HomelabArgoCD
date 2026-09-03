# ---- Build stage ----
FROM golang:1.27.1-alpine3.24 AS builder
WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod ./

# Copy the Go source
COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o mimir-redirect .

# ---- Final stage: distroless static, no shell, no libc, no interpreter ----
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot AS base
WORKDIR /
COPY --from=builder /workspace/mimir-redirect .
USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/mimir-redirect"]
