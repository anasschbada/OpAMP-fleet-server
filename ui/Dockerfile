# Builds the static UI bundle (Vite) and the tiny Go static-file server
# that serves it (cmd/fleet-ui-server), on the same hardened base image
# family as the main server's Dockerfile. See that file's header comment
# for the airgapped-registry mirroring and digest-pinning notes -- they
# apply here too. Also no HEALTHCHECK, for the same reason (Kubernetes
# probes gate this workload, see deploy/k8s/platform/11-ui-deployment.yaml).
#
# Build from the REPO ROOT, not this directory, since this Dockerfile also
# needs the top-level go.mod/go.sum and cmd/fleet-ui-server:
#   docker build -f ui/Dockerfile -t opamp-fleet-ui:latest .

ARG NODE_IMAGE=node:22-bookworm
ARG GO_IMAGE=golang:1.25-bookworm

FROM ${NODE_IMAGE} AS build-ui
WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json ./
# npm ci (not install): fails if package-lock.json is out of sync with
# package.json instead of silently rewriting the lockfile, and only ever
# installs exactly what's pinned there.
RUN npm ci
COPY ui/ .
RUN npm run build

FROM ${GO_IMAGE} AS build-server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/fleet-ui-server/ cmd/fleet-ui-server/

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/fleet-ui-server ./cmd/fleet-ui-server

FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="opamp-fleet-ui" \
      org.opencontainers.image.description="Static file server for the OpAMP fleet management UI" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

COPY --from=build-server /out/fleet-ui-server /fleet-ui-server
COPY --from=build-ui /src/ui/dist /www
USER 65532:65532
ENV STATIC_DIR=/www
ENTRYPOINT ["/fleet-ui-server"]
