package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/bitnami-labs/flagenv"
	pb "github.com/mkmik/simplegrpc/helloworld"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"google.golang.org/grpc/binarylog"
	channelz "google.golang.org/grpc/channelz/service"
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

func init() {
	sink, err := binarylog.NewTempFileSink()
	if err != nil {
		panic(err)
	}
	binarylog.SetSink(sink)
	grpc.EnableTracing = true
}

func mainE() error {

	log.Printf("Serving http at %q", ":2211")
	go http.ListenAndServe(":2211", nil)

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
	channelz.RegisterChannelzServiceToServer(srv)

	log.Printf("Serving id %q gRPC at %q", *id, listenAddr)
	return srv.Serve(listener)
}

func main() {
	flag.Parse()

	if err := mainE(); err != nil {
		log.Fatal(err)
	}
}
