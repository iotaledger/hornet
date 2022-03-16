# https://hub.docker.com/_/golang
FROM golang:1.17-bullseye AS build

ARG BUILD_TAGS=builtin_static,rocksdb

LABEL org.label-schema.description="HORNET - The IOTA community node"
LABEL org.label-schema.name="gohornet/hornet"
LABEL org.label-schema.schema-version="1.0"
LABEL org.label-schema.vcs-url="https://github.com/gohornet/hornet"
LABEL org.label-schema.usage="https://github.com/gohornet/hornet/blob/main/documentation/docs/getting_started/using_docker.md"

# Ensure ca-certificates are up to date
RUN update-ca-certificates

# Set the current Working Directory inside the container
RUN mkdir /scratch
WORKDIR /scratch

# Prepare the folder where we are putting all the files
RUN mkdir /app

# Copy everything from the current directory to the PWD(Present Working Directory) inside the container
COPY . .

# Download go modules
RUN go mod download
RUN go mod verify

# Build the binary
RUN go build -o /app/hornet -a -tags="$BUILD_TAGS" -ldflags='-w -s'

# Build the Indexer INX binary
RUN mkdir -p /app/inx/indexer
RUN go build -o /app/inx/indexer/inx-indexer ./tools/inx-indexer
COPY ./inx/indexer/inx.json /app/inx/indexer/inx.json

# Build the MQTT INX binary
RUN mkdir -p /app/inx/mqtt
RUN go build -o /app/inx/mqtt/inx-mqtt ./tools/inx-mqtt
COPY ./inx/mqtt/inx.json /app/inx/mqtt/inx.json

# Copy the assets
COPY ./config.json /app/config.json
COPY ./peering.json /app/peering.json
COPY ./profiles.json /app/profiles.json

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

# Copy the app dir into distroless image
COPY --chown=nonroot:nonroot --from=build /app /app

WORKDIR /app
USER nonroot

ENTRYPOINT ["/app/hornet"]
