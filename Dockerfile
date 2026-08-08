# The builder runs on the machine's own architecture and cross-compiles for the target
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Reported by GET /version. The default keeps a plain `docker build` working
ARG VERSION=dev
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags "-s -w -X librelock-server/version.Version=${VERSION}" \
    -o server .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8000
CMD ["./server"]
