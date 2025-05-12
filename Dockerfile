
FROM golang:1.24 AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
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
RUN GOOS=linux go build -tags netgo --ldflags "-extldflags -static" -o server cmd/kafkalayer/main.go
# if it compiles, run unit tests
RUN go vet ./... && go test -v ./...

# Build final image
FROM gcr.io/distroless/static-debian12:nonroot AS final

COPY --from=build /app/server .

EXPOSE 8080

CMD ["./server"]
