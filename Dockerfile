# https://hub.docker.com/_/golang
FROM golang:1.19-bullseye AS build

ARG BUILD_TAGS=rocksdb

LABEL org.label-schema.description="HORNET - The IOTA node"
LABEL org.label-schema.name="iotaledger/hornet"
LABEL org.label-schema.schema-version="1.0"
LABEL org.label-schema.vcs-url="https://github.com/iotaledger/hornet"

# Ensure ca-certificates are up to date
RUN update-ca-certificates

# Set the current Working Directory inside the container
RUN mkdir /scratch
WORKDIR /scratch

# Prepare the folder where we are putting all the files
RUN mkdir /app

# Make sure that modules only get pulled when the module file has changed
COPY go.mod go.sum ./

# Download go modules
RUN go mod download
RUN go mod verify

# Copy everything from the current directory to the PWD(Present Working Directory) inside the container
COPY . .

# Build the binary
RUN go build -o /app/hornet -a -tags="$BUILD_TAGS" -ldflags='-w -s'

# Copy the assets
COPY ./config_defaults.json /app/config.json
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
EXPOSE 9029/tcp

HEALTHCHECK --interval=10s --timeout=5s --retries=30 CMD ["/app/hornet", "tools", "node-info"]

# Copy the app dir into distroless image
COPY --chown=nonroot:nonroot --from=build /app /app

WORKDIR /app
USER nonroot

ENTRYPOINT ["/app/hornet"]
