.PHONY: build run proto

build:
	go build ./cmd/main.go

run:
	./main

proto:
	protoc -I=../daemon/protobuf \
	  --go_out=./internal/pb --go_opt=paths=source_relative \
	  --go-grpc_out=./internal/pb --go-grpc_opt=paths=source_relative \
	  metrics.proto
