package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/bitnami-labs/flagenv"
	"github.com/mkmik/simplegrpc/helloworld"
	pb "github.com/mkmik/simplegrpc/helloworld"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"google.golang.org/grpc/binarylog"
	channelz "google.golang.org/grpc/channelz/service"
)

var (
	id     = flag.String("id", "", "task identifier")
	client = flag.String("client", "", "connect to addr")
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

	time.Sleep(1 * time.Second)

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

func clientE(addr string) error {
	cli, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx, _ = context.WithTimeout(ctx, 500*time.Millisecond)

	log.Printf("calling")
	hello := helloworld.NewGreeterClient(cli)
	res, err := hello.SayHello(ctx, &helloworld.HelloRequest{Name: "foo"})
	if err != nil {
		return err
	}
	log.Println(res)

	return nil
}

func serverE() error {
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

	// flush the grpc binary log sink
	defer binarylog.SetSink(nil)

	if *client != "" {
		if err := clientE(*client); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	} else {
		if err := serverE(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	}
}
