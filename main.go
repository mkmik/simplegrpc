package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bitnami-labs/flagenv"
	"github.com/mkmik/simplegrpc/helloworld"
	"github.com/mkmik/simplegrpc/rotatingbinarylog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"google.golang.org/grpc/binarylog"
	channelz "google.golang.org/grpc/channelz/service"
)

var (
	id         = flag.String("id", "", "task identifier")
	client     = flag.String("client", "", "connect to addr")
	message    = flag.String("message", "foo", "message to send")
	iterations = flag.Int("iterations", 1000, "client iterations")

	usebinlog = flag.Bool("binary-log-enable", true, "Enable binary log")
	maxSize   = flag.Uint64("binary-log-max-size", 0, "binary log max-size")
	rotate    = flag.Int("binary-log-rotate", 1, "Log files are rotated n times before being removed. If 0, old files are never deleted")
	filename  = flag.String("binary-log-filename", rotatingbinarylog.DefaultFilename, "Binary log file name")
)

func init() {
	flagenv.SetFlagsFromEnv("SIMPLE_GRPC", flag.CommandLine)
}

// server is used to implement helloworld.GreeterServer.
type server struct {
	helloworld.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	if false {
		log.Printf("Received: %v", in.GetName())
	}

	//	time.Sleep(1 * time.Second)

	return &helloworld.HelloReply{Message: fmt.Sprintf("Hello %s from %s", in.GetName(), *id)}, nil
}

func initGRPC(usebinlog bool, filename string, sizeLimit uint64, rotate int) {
	if usebinlog {
		sink, err := rotatingbinarylog.NewSink(
			rotatingbinarylog.WithFilename(filename),
			rotatingbinarylog.WithMaxSize(sizeLimit),
			rotatingbinarylog.WithMaxRotations(rotate),
		)
		if err != nil {
			panic(err)
		}
		binarylog.SetSink(sink)
	}
	grpc.EnableTracing = true
}

func clientE(addr, msg string, iterations int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("caught signal %v\n", sig)
		cancel()
	}()

	numCalls := 0
	defer func() {
		log.Printf("performed %d calls", numCalls)
	}()

	cli, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	//	ctx, _ = context.WithTimeout(ctx, 500*time.Millisecond)

	start := time.Now()
	for i := 0; i < iterations; i++ {
		numCalls++

		if false {
			log.Printf("calling")
		}
		hello := helloworld.NewGreeterClient(cli)
		res, err := hello.SayHello(ctx, &helloworld.HelloRequest{Name: msg})
		if err != nil {
			return err
		}
		if false {
			log.Println(res)
		}
	}
	elapsed := time.Since(start)
	log.Printf("elapsed: %s RPS: %f", elapsed, float64(iterations)/(float64(elapsed)/float64(time.Second)))

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
	helloworld.RegisterGreeterServer(srv, &server{})
	channelz.RegisterChannelzServiceToServer(srv)

	log.Printf("Serving id %q gRPC at %q", *id, listenAddr)
	return srv.Serve(listener)
}

func main() {
	flag.Parse()

	initGRPC(*usebinlog, *filename, *maxSize, *rotate)

	// flush the grpc binary log sink
	defer binarylog.SetSink(nil)

	if *client != "" {
		if err := clientE(*client, *message, *iterations); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	} else {
		if err := serverE(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	}
}
