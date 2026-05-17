FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go test ./...
ARG VERSION=dev
RUN go build -ldflags "-X main.version=${VERSION}" -o pingsvc .

FROM alpine:3.20
RUN adduser -D -u 1001 app
USER app
WORKDIR /app
COPY --from=builder /app/pingsvc .
EXPOSE 8080
ENTRYPOINT ["./pingsvc"]
