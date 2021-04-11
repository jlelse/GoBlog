FROM golang:1.16-alpine3.13 as build
WORKDIR /app
RUN apk add --no-cache git gcc musl-dev sqlite-dev
ADD *.go go.mod go.sum /app/
RUN go build --tags "libsqlite3 linux sqlite_fts5"

FROM alpine:3.13
WORKDIR /app
VOLUME /app/config
VOLUME /app/data
EXPOSE 80
EXPOSE 443
EXPOSE 8080
CMD ["GoBlog"]
HEALTHCHECK --interval=1m --timeout=10s CMD GoBlog healthcheck
RUN apk add --no-cache sqlite-dev tzdata tor
COPY templates/ /app/templates/
COPY --from=build /app/GoBlog /bin/