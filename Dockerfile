FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application (strip debug info for smaller size)
RUN go build -trimpath -ldflags="-s -w" -o lfscache .

FROM alpine:latest

# TODO(jamison): error: failed to solve: executor failed running [/bin/sh -c apk --no-cache add ca-certificates]: exit code: 1
#RUN apk --no-cache add ca-certificates

COPY --from=builder /app/lfscache /bin/lfscache

ENTRYPOINT ["/bin/lfscache"]
