FROM --platform=$BUILDPLATFORM golang:1.26.9-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal ./internal
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/opentofu-test-mcp .

FROM ghcr.io/opentofu/opentofu:1.13.1
ARG VERSION=dev
LABEL org.opencontainers.image.source="https://github.com/ogormans-deptstack/opentofu-test-mcp" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      io.modelcontextprotocol.server.name="io.github.ogormans-deptstack/opentofu-test"
COPY --from=build /out/opentofu-test-mcp /usr/local/bin/opentofu-test-mcp
WORKDIR /workspace
ENTRYPOINT ["opentofu-test-mcp"]
