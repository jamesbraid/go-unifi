# syntax=docker/dockerfile:1.20@sha256:26147acbda4f14c5add9946e2fd2ed543fc402884fd75146bd342a7f6271dc1d
ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG GO_IMAGE=golang:1.26.5-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651

FROM --platform=$BUILDPLATFORM $GO_IMAGE AS dependencies

WORKDIR /deps
COPY go.mod go.sum ./
RUN go mod download all && go mod verify

FROM $GO_IMAGE
COPY --from=dependencies /go/pkg/mod /go/pkg/mod
COPY build/m0/container-rebuild /usr/local/bin/go-unifi-container-rebuild
RUN chmod 0755 /usr/local/bin/go-unifi-container-rebuild

ENV GOPROXY=off \
    GOTOOLCHAIN=local \
    GOTELEMETRY=off

ENTRYPOINT ["/usr/local/bin/go-unifi-container-rebuild"]
