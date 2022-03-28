# https://hub.docker.com/_/golang
FROM golang:1.18-bullseye AS build

ARG BUILD_TAGS=builtin_static,rocksdb

LABEL org.label-schema.description="HORNET - The IOTA community node"
LABEL org.label-schema.name="gohornet/hornet"
LABEL org.label-schema.schema-version="1.0"
LABEL org.label-schema.vcs-url="https://github.com/gohornet/hornet"
LABEL org.label-schema.usage="https://github.com/gohornet/hornet/blob/main/documentation/docs/getting_started/using_docker.md"

# Ensure ca-certificates are up to date
RUN update-ca-certificates

# Set the current Working Directory inside the container
RUN mkdir /app
WORKDIR /app

# Use Go Modules
COPY go.mod .
COPY go.sum .

ENV GO111MODULE=on
RUN go mod download
RUN go mod verify

# Copy everything from the current directory to the PWD(Present Working Directory) inside the container
COPY . .

# Build the binary
RUN go build \
      -tags="$BUILD_TAGS" \
      -ldflags='-w -s' -a \
      -o /go/bin/hornet

############################
# Image
############################
# https://console.cloud.google.com/gcr/images/distroless/global/cc-debian11
# using distroless cc "nonroot" image, which includes everything in the base image (glibc, libssl and openssl)
FROM gcr.io/distroless/cc-debian11:nonroot

EXPOSE 15600/tcp
EXPOSE 14626/udp
EXPOSE 14265/tcp
EXPOSE 8081/tcp
EXPOSE 8091/tcp
EXPOSE 1883/tcp

# Copy the binary into distroless image
COPY --chown=nonroot:nonroot --from=build /go/bin/hornet /app/hornet

# Copy the assets
COPY ./config.json /app/config.json
COPY ./config_devnet.json /app/config_devnet.json
COPY ./peering.json /app/peering.json
COPY ./profiles.json /app/profiles.json

WORKDIR /app
USER nonroot

ENTRYPOINT ["/app/hornet"]
