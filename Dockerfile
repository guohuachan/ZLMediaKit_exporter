# pre-build
# ADD sources.list /etc/apt/sources.list,if network is blocked
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY . /app

RUN go build -o zlm_exporter .

# post-build
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/zlm_exporter .

EXPOSE 9101

ENTRYPOINT ["/app/zlm_exporter"]