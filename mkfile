#!/bin/env bash
set -xe

d:
  go run cmd/app/main.go

b:
  go build -o bin/app cmd/app/main.go

m:
  migrate create -ext sql -dir migrations -seq $1

t:
  go test ./...

lint:
  golangci-lint run ./...
