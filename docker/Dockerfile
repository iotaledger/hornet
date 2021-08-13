# https://hub.docker.com/_/golang
# golang 1.16.7-buster amd64
FROM golang@sha256:448a13037d13401ad9b31fabf91d6b8a3c5c35d336cc8af760b2ab4ed85d4155 AS build

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
RUN GOOS=linux GOARCH=amd64 go build \
      -tags="$BUILD_TAGS" \
      -ldflags='-w -s' -a \
      -o /go/bin/hornet

############################
# Image
############################
# https://github.com/GoogleContainerTools/distroless/tree/master/cc
# using distroless cc "nonroot" image, which includes everything in the base image (glibc, libssl and openssl)
FROM gcr.io/distroless/cc@sha256:1a125767c4a066c5c2c70b326bcc3cff8ae4260de0b488ddcfdfe84f20a373b4

EXPOSE 15600/tcp
EXPOSE 14626/udp
EXPOSE 14265/tcp
EXPOSE 8081/tcp
EXPOSE 1883/tcp

# Copy the binary into distroless image
COPY --chown=nonroot:nonroot --from=build /go/bin/hornet /app/hornet

# Copy the assets
COPY ./config.json /app/config.json
COPY ./config_comnet.json /app/config_comnet.json
COPY ./peering.json /app/peering.json
COPY ./profiles.json /app/profiles.json

WORKDIR /app
USER nonroot

ENTRYPOINT ["/app/hornet"]
