FROM golang:1.25-alpine3.23 AS buildbase

WORKDIR /app
RUN apk add --no-cache git gcc musl-dev
RUN apk add --no-cache --repository=https://dl-cdn.alpinelinux.org/alpine/edge/main sqlite-dev
ENV GOFLAGS="-tags=linux,libsqlite3,sqlite_fts5,skipIntegration"
ADD *.go go.mod go.sum /app/
ADD pkgs/ /app/pkgs/
ADD testdata/ /app/testdata/
ADD templates/ /app/templates/
ADD leaflet/ /app/leaflet/
ADD hlsjs/ /app/hlsjs/
ADD dbmigrations/ /app/dbmigrations/
ADD strings/ /app/strings/
ADD plugins/ /app/plugins/
ADD logo/ /app/logo/

FROM buildbase AS build

RUN go build -ldflags '-w -s' -o GoBlog

FROM build AS test

RUN go test -timeout 300s -failfast -cover ./...

FROM alpine:3.23 AS base

WORKDIR /app
VOLUME /app/config
VOLUME /app/data
EXPOSE 80
EXPOSE 443
EXPOSE 8080
CMD ["GoBlog"]
HEALTHCHECK --interval=1m --timeout=10s CMD GoBlog healthcheck
ENV GOMEMLIMIT=100MiB
RUN apk add --no-cache tzdata tor
RUN apk add --no-cache --repository=https://dl-cdn.alpinelinux.org/alpine/edge/main sqlite-dev
COPY templates/ /app/templates/
COPY --from=build /app/GoBlog /bin/

FROM base AS tools

RUN apk add --no-cache curl bash git
RUN apk add --no-cache --repository=https://dl-cdn.alpinelinux.org/alpine/edge/main sqlite