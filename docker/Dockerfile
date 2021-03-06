# https://hub.docker.com/_/golang
# golang 1.16.4-buster amd64
FROM golang@sha256:f7a5c5872d4bb68e152be72e4a4bf9a142a47ec2dcbb4074798d4feb6197abd7 AS build

ARG BUILD_TAGS=builtin_static,rocksdb

LABEL org.label-schema.description="HORNET - The IOTA community node"
LABEL org.label-schema.name="iotaledger/hornet"
LABEL org.label-schema.schema-version="1.0"
LABEL org.label-schema.vcs-url="https://github.com/iotaledger/hornet"
LABEL org.label-schema.usage="https://github.com/iotaledger/hornet/blob/master/DOCKER.md"

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
FROM alpine:latest

EXPOSE 8081/tcp
EXPOSE 14265/tcp
EXPOSE 15600/tcp
EXPOSE 1883/tcp

# Copy the binary into base image
RUN addgroup --gid 39999 hornet \
 && adduser -h /app -s /bin/sh -G hornet -u 39999 -D hornet 

RUN apk add --no-cache gcompat libc6-compat libstdc++ libgcc openssl openssl-dev

COPY --chown=hornet:hornet --from=build /go/bin/hornet /app/hornet

# Copy the assets
COPY ./config.json /app/config.json
COPY ./config_as.json /app/config_as.json
COPY ./peering.json /app/peering.json
COPY ./profiles.json /app/profiles.json

WORKDIR /app
USER hornet

ENTRYPOINT ["/app/hornet"]
