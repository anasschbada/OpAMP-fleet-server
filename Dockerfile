# Builds the opamp-server binary as a static, CGO-free executable and ships
# it on a distroless, non-root, shell-less base image.
#
# Airgapped registries: both base images below must be mirrored into your
# internal registry first (this Dockerfile cannot reach docker.io/gcr.io
# from inside an airgapped cluster) -- pull, retag, push to your registry,
# then override with --build-arg GO_IMAGE=... and change the final FROM
# line to your mirrored distroless tag.

ARG GO_IMAGE=golang:1.24-bookworm

FROM ${GO_IMAGE} AS build
WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

# CGO_ENABLED=0: modernc.org/sqlite is pure Go, so this produces a fully
# static binary with no libc dependency -- required for the distroless
# "static" base image below, and it means there is no C toolchain/libc CVE
# surface to track in the shipped image at all.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/opamp-server ./cmd/opamp-server

# distroless/static: no shell, no package manager, no libc -- just enough
# to run a static Go binary as a non-root user (nonroot tag = uid/gid 65532,
# matching deploy/k8s/platform/06-deployment.yaml's runAsUser).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/opamp-server /opamp-server
USER 65532:65532
ENTRYPOINT ["/opamp-server"]
