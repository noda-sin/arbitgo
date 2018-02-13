#!/bin/bash
GOOS=linux GOARCH=amd64 go build -o ./build/arbitgo main.go
docker build -t arbitgo .