# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
FROM golang:1.18 as builder
RUN apt-get update && apt-get -y install docker-compose
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download


# Copy the source from the current directory to the Working Directory inside the container
# Make available as separate stage for building test-runner image
FROM builder AS src
COPY *.go ./
COPY cmd ./cmd
COPY internal ./internal
COPY resources ./resources


# Compile binary
FROM src AS build
RUN CGO_ENABLED=1 GOOS=linux go build -a -o server cmd/kafkalayer/main.go
RUN go vet ./...
RUN go test ./... -v


# Build final image
FROM alpine:latest

RUN apk --no-cache add ca-certificates
RUN wget -q -O /etc/apk/keys/sgerrand.rsa.pub https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub && \
    wget https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.32-r0/glibc-2.32-r0.apk && \
    apk add glibc-2.32-r0.apk

WORKDIR /root/

COPY --from=build /app/server .

EXPOSE 8080

CMD ["./server"]
