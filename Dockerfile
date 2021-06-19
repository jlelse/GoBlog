FROM golang:1.16-alpine3.13 as build
WORKDIR /app
RUN apk add --no-cache git gcc musl-dev
RUN apk add --no-cache --repository=http://dl-cdn.alpinelinux.org/alpine/edge/main sqlite-dev
ENV GOFLAGS="-tags=linux,libsqlite3,sqlite_fts5"
ADD *.go go.mod go.sum /app/
ADD pkgs/ /app/pkgs/
ADD testdata/ /app/testdata/
Add templates/ /app/templates/
RUN go test -cover ./...
RUN go build -ldflags '-w -s' -o GoBlog

FROM alpine:3.13
WORKDIR /app
VOLUME /app/config
VOLUME /app/data
EXPOSE 80
EXPOSE 443
EXPOSE 8080
CMD ["GoBlog"]
HEALTHCHECK --interval=1m --timeout=10s CMD GoBlog healthcheck
RUN apk add --no-cache tzdata tor
RUN apk add --no-cache --repository=http://dl-cdn.alpinelinux.org/alpine/edge/main sqlite-dev
COPY templates/ /app/templates/
COPY --from=build /app/GoBlog /bin/