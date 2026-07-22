# Builds the opamp-server binary as a static, CGO-free executable and ships
# it on a distroless, non-root, shell-less base image.
#
# Airgapped registries: both base images below must be mirrored into your
# internal registry first (this Dockerfile cannot reach docker.io/gcr.io
# from inside an airgapped cluster) -- pull, retag, push to your registry,
# then override with --build-arg GO_IMAGE=... and change the final FROM
# line to your mirrored distroless tag. Once mirrored, pin both to a
# digest (not just a tag) for reproducible builds:
#   docker inspect --format='{{index .RepoDigests 0}}' <image>
#
# No HEALTHCHECK instruction: this image only ever runs under Kubernetes,
# whose own liveness/readiness probes (deploy/k8s/platform/07-deployment.yaml,
# hitting /healthz and /readyz) are what actually gate traffic and restarts.
# A Docker-level HEALTHCHECK would need a shell/curl binary anyway, neither
# of which exists on the distroless runtime image below.

ARG GO_IMAGE=golang:1.24-bookworm

FROM ${GO_IMAGE} AS build
WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

# TARGETOS/TARGETARCH are populated automatically by BuildKit (docker
# buildx build --platform ...) for multi-arch builds; default to the
# builder's own platform for a plain `docker build`.
ARG TARGETOS
ARG TARGETARCH

# CGO_ENABLED=0: modernc.org/sqlite is pure Go, so this produces a fully
# static binary with no libc dependency -- required for the distroless
# "static" base image below, and it means there is no C toolchain/libc CVE
# surface to track in the shipped image at all.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/opamp-server ./cmd/opamp-server

# distroless/static: no shell, no package manager, no libc -- just enough
# to run a static Go binary as a non-root user (nonroot tag = uid/gid 65532,
# matching deploy/k8s/platform/07-deployment.yaml's runAsUser).
FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="opamp-fleet-server" \
      org.opencontainers.image.description="OpAMP protocol server + REST API for OpenTelemetry Collector fleet management" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

COPY --from=build /out/opamp-server /opamp-server
USER 65532:65532
ENTRYPOINT ["/opamp-server"]
