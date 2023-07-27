# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
FROM golang:1.20-alpine as builder
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
RUN apk update && \
    apk upgrade && \
    apk add pkgconf git bash build-base sudo librdkafka-dev pkgconf ca-certificates
RUN GOOS=linux GOARCH=amd64 go build -tags musl --ldflags "-extldflags -static" -o server cmd/kafkalayer/main.go
RUN go vet ./... && go test -tags musl -v ./...


# Build final image
FROM scratch AS final
WORKDIR /root
COPY --from=build /app/server .

EXPOSE 8080

CMD ["./server"]
