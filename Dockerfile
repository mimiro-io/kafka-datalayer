# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
FROM --platform=linux/amd64 golang:1.22.2 as builder

RUN apt-get update && \
    apt-get install git ca-certificates gcc -y && \
    update-ca-certificates

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

# let all users in group 0 run the binary
RUN chgrp 0 server && chmod g+X server


# Build final image
FROM scratch AS final

WORKDIR /root

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app/server /root/server

EXPOSE 8080

# set non-root user
USER 5678

CMD ["/root/server"]
