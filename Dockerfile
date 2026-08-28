# Build stage.
#
# Behind a restricted network, pass a module proxy:
#   docker build --build-arg GOPROXY=https://goproxy.cn,direct .
FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BRANCH=unknown
ARG BUILD_DATE=unknown
ARG GOPROXY

WORKDIR /app

# Download dependencies first so the layer is cached across source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="-s -w \
        -X github.com/prometheus/common/version.Version=${VERSION} \
        -X github.com/prometheus/common/version.Revision=${COMMIT_SHA} \
        -X github.com/prometheus/common/version.Branch=${BRANCH} \
        -X github.com/prometheus/common/version.BuildDate=${BUILD_DATE} \
        -X main.BuildVersion=${VERSION} \
        -X main.BuildCommitSha=${COMMIT_SHA} \
        -X main.BuildDate=${BUILD_DATE}" \
      -o /out/zlm_exporter .

# Runtime stage
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
 && adduser -D -H -u 65532 zlmexporter

COPY --from=builder /out/zlm_exporter /usr/local/bin/zlm_exporter

USER zlmexporter
EXPOSE 9101

ENTRYPOINT ["/usr/local/bin/zlm_exporter"]
