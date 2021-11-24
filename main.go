package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/bitnami-labs/flagenv"
	pb "github.com/mkmik/simplegrpc/helloworld"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var (
	id = flag.String("id", "", "task identifier")
)

func init() {
	flagenv.SetFlagsFromEnv("SIMPLE_GRPC", flag.CommandLine)
}

// server is used to implement helloworld.GreeterServer.
type server struct {
	pb.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: fmt.Sprintf("Hello %s from %s", in.GetName(), *id)}, nil
}

func mainE() error {
	const listenAddr = ":1122"
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	srv := grpc.NewServer()
	hserver := health.NewServer()
	reflection.Register(srv)
	grpc_health_v1.RegisterHealthServer(srv, hserver)
	pb.RegisterGreeterServer(srv, &server{})

	log.Printf("Serving gRPC at %q", listenAddr)
	return srv.Serve(listener)
}

func main() {
	flag.Parse()

	if err := mainE(); err != nil {
		log.Fatal(err)
	}
}
