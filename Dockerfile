FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /build

ARG TARGETOS=linux
ARG TARGETARCH

# Copy go module files first for better caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

RUN apk update && apk add upx
# Copy the rest
COPY . .

ARG VERSION=main
ENV CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -a -installsuffix cgo -ldflags="-w -s -X github.com/bakito/vault-unsealer/version.Version=${VERSION}" -o vault-unsealer main.go && \
    upx -q vault-unsealer

# application image

FROM scratch

LABEL maintainer="bakito <github@bakito.ch>"
EXPOSE 8080 8090 9153
WORKDIR /opt/go/
USER 1001
ENTRYPOINT ["/opt/go/vault-unsealer"]

COPY --from=builder /build/vault-unsealer /opt/go/vault-unsealer
