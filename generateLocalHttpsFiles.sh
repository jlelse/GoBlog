#!/usr/bin/env sh

openssl genrsa -out https/server.key 2048
openssl ecparam -genkey -name secp384r1 -out https/server.key
openssl req -new -x509 -sha256 -key https/server.key -out https/server.crt -days 3650