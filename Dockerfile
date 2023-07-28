# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
FROM golang:1.20 as builder

RUN apt-get update && \
    apt-get install git ca-certificates gcc -y && \
    update-ca-certificates

ENV USER=appuser
ENV UID=10001
# See https://stackoverflow.com/a/55757473/12429735RUN
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"
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
RUN GOOS=linux GOARCH=amd64 go build -tags netgo --ldflags "-extldflags -static" -o server cmd/kafkalayer/main.go
# if it compiles, run unit tests
RUN go vet ./... && go test -v ./...


# Build final image
FROM scratch AS final

WORKDIR /root

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group /etc/group
COPY --from=build /app/server /root/server

EXPOSE 8080

USER appuser:appuser
CMD ["/root/server"]
