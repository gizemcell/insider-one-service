FROM golang:1.22-alpine3.21 AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go test ./...
ARG VERSION=dev
RUN go build -ldflags "-X main.version=${VERSION}" -o insider-service .

FROM alpine:3.21
RUN adduser -D -u 1001 app
USER app
WORKDIR /app
COPY --from=builder /app/insider-service .
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --spider -q http://localhost:8080/healthz || exit 1
ENTRYPOINT ["./insider-service"]
