# Builds the static UI bundle (Vite) and the tiny Go static-file server
# that serves it (cmd/fleet-ui-server), on the same hardened base image
# family as the main server's Dockerfile. See that file's header comment
# for the airgapped-registry mirroring note -- it applies here too.
#
# Build from the REPO ROOT, not this directory, since this Dockerfile also
# needs the top-level go.mod/go.sum and cmd/fleet-ui-server:
#   docker build -f ui/Dockerfile -t opamp-fleet-ui:latest .

ARG NODE_IMAGE=node:22-bookworm
ARG GO_IMAGE=golang:1.24-bookworm

FROM ${NODE_IMAGE} AS build-ui
WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ .
RUN npm run build

FROM ${GO_IMAGE} AS build-server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/fleet-ui-server/ cmd/fleet-ui-server/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fleet-ui-server ./cmd/fleet-ui-server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build-server /out/fleet-ui-server /fleet-ui-server
COPY --from=build-ui /src/ui/dist /www
USER 65532:65532
ENV STATIC_DIR=/www
ENTRYPOINT ["/fleet-ui-server"]
