FROM golang:1.20 as builder
WORKDIR /build

RUN apt-get update && apt-get install -y upx
COPY . .

ARG VERSION=main
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN go build -a -installsuffix cgo -ldflags="-w -s -X github.com/bakito/vault-unsealer/version.Version=${VERSION}" -o vault-unsealer main.go && \
    upx -q vault-unsealer

# application image

FROM scratch

LABEL maintainer="bakito <github@bakito.ch>"
EXPOSE 8080 8090 9153
WORKDIR /opt/go/
USER 1001
ENTRYPOINT ["/opt/go/vault-unsealer"]

COPY --from=builder /build/vault-unsealer /opt/go/vault-unsealer
