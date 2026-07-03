#!/usr/bin/env bash

# 1. Create a Private Certificate Authority (CA)
openssl req -x509 -newkey rsa:4096 -keyout ca.key -out ca.crt -days 365 -nodes -subj "/CN=MyClusterCA"

# 2. Generate Server Private Key and Certificate (Signed by your CA)
openssl req -newkey rsa:2048 -keyout server.key -out server.csr -nodes -subj "/CN=localhost"
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365

# 3. Generate Client Private Key and Certificate (Signed by your CA)
openssl req -newkey rsa:2048 -keyout client.key -out client.csr -nodes -subj "/CN=react-backend-client"
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 365
