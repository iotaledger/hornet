# golang 1.14.4-buster
FROM golang@sha256:ebe7f5d1a2a6b884bc1a45b8c1ff7e26b7b95938a3e8847ea96fc6761fdc2b77 AS build

# Ensure ca-certficates are up to date
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
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags="pow_avx" \
      -ldflags='-w -s -extldflags "-static"' -a \
       -o /go/bin/hornet

############################
# Image
############################
FROM alpine:latest

EXPOSE 8081/tcp
EXPOSE 14265/tcp
EXPOSE 15600/tcp
EXPOSE 14626/udp

# Copy assets into distroless image
COPY --from=build /go/bin/hornet /app/hornet
COPY ./config.json /app/config.json
COPY ./config_comnet.json /app/config_comnet.json
COPY ./config_devnet.json /app/config_devnet.json
COPY ./peering.json /app/peering.json
COPY ./profiles.json /app/profiles.json
COPY ./mqtt_config.json /app/mqtt_config.json

RUN addgroup --gid 39999 hornet \
 && adduser -h /app -s /bin/sh -G hornet -u 39999 -D hornet \
 && chown hornet:hornet -R /app

WORKDIR "/app"
USER hornet

ENTRYPOINT ["/app/hornet"]
