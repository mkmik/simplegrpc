//go:generate protoc --go_out=. --go-grpc_out=. --go_opt=module=github.com/mkmik/simplegrpc/helloworld  --go-grpc_opt=module=github.com/mkmik/simplegrpc/helloworld helloworld.proto

package helloworld
