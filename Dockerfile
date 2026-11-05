# syntax=docker/dockerfile:1
FROM node:24-alpine AS frontend
WORKDIR /src
COPY web/package*.json ./web/
RUN --mount=type=cache,target=/root/.npm cd web && npm ci --no-audit --no-fund
COPY web ./web
RUN cd web && npm run build

FROM golang:1.27.3-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/internal/ui/dist ./internal/ui/dist
RUN --mount=type=cache,target=/root/.cache/go-build go generate ./internal/startup
ARG VERSION=1.0.0-rc.6
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg/mod CGO_ENABLED=0 go build -p 2 -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /shipper ./cmd/shipper

FROM alpine:3.23
RUN apk add --no-cache ca-certificates git docker-cli docker-cli-buildx docker-cli-compose && mkdir /data && chown 65532:65532 /data
COPY --from=backend /shipper /usr/local/bin/shipper
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/licenses/shipper/
USER 65532:65532
ENV SHIPPER_ADDR=0.0.0.0:8080 SHIPPER_DATA_DIR=/data
EXPOSE 8080
VOLUME /data
HEALTHCHECK --interval=30s --timeout=4s --start-period=5s CMD ["shipper", "healthcheck"]
ENTRYPOINT ["shipper"]

