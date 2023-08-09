FROM golang:1.21-alpine3.18 as buildbase

WORKDIR /app
RUN apk add --no-cache git gcc musl-dev
RUN apk add --no-cache --repository=http://dl-cdn.alpinelinux.org/alpine/edge/main sqlite-dev
ENV GOFLAGS="-tags=linux,libsqlite3,sqlite_fts5"
ADD *.go go.mod go.sum /app/
ADD pkgs/ /app/pkgs/
ADD testdata/ /app/testdata/
ADD templates/ /app/templates/
ADD leaflet/ /app/leaflet/
ADD hlsjs/ /app/hlsjs/
ADD dbmigrations/ /app/dbmigrations/
ADD strings/ /app/strings/
ADD plugins/ /app/plugins/
ADD logo/GoBlog.png /app/logo/GoBlog.png

FROM buildbase as build

RUN go build -ldflags '-w -s' -o GoBlog

FROM build as test

RUN go test -timeout 300s -failfast -cover ./...

FROM alpine:3.18 as base

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
RUN apk add --no-cache --repository=http://dl-cdn.alpinelinux.org/alpine/edge/main sqlite-dev
COPY templates/ /app/templates/
COPY --from=build /app/GoBlog /bin/

FROM base as tools

RUN apk add --no-cache curl bash git
RUN apk add --no-cache --repository=http://dl-cdn.alpinelinux.org/alpine/edge/main sqlite