# syntax=docker/dockerfile:1
#
# Multi-arch, cgo-free static build. The build stage is pinned to the builder's
# native platform and Go cross-compiles to the target arch (no QEMU needed).
#
# procfit observes the host: share the host PID namespace so its /proc shows host
# processes, e.g.
#   docker run --rm --pid=host ghcr.io/sparrow-in-code/procfit ps
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS TARGETARCH VERSION=dev COMMIT=none DATE=unknown
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w \
      -X github.com/netikras/procfit/internal/meta.Version=${VERSION} \
      -X github.com/netikras/procfit/internal/meta.Commit=${COMMIT} \
      -X github.com/netikras/procfit/internal/meta.BuildDate=${DATE}" \
    -o /out/procfit ./cmd/procfit

FROM scratch
COPY --from=build /out/procfit /procfit
ENTRYPOINT ["/procfit"]
