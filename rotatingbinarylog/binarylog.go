package rotatingbinarylog

import (
	"log"
	"sync"

	"github.com/mkmik/simplegrpc/rotatingbinarylog/internal/sink"
	"google.golang.org/grpc/binarylog"
	pb "google.golang.org/grpc/binarylog/grpc_binarylog_v1"
	"google.golang.org/protobuf/proto"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Sink struct {
	sync.Mutex

	rotate      func() error
	currentSink binarylog.Sink
	currentSize uint64

	sizeLimit uint64
}

type Flusher interface {
	Flush() error
}

type NewTempFileSinkOption func(*newTmpFileSinkOptions)

type newTmpFileSinkOptions struct {
	sizeLimit uint64
}

func WithSizeLimit(sizeLimit uint64) NewTempFileSinkOption {
	return func(opt *newTmpFileSinkOptions) {
		opt.sizeLimit = sizeLimit
	}
}

func NewTempFileSink(opts ...NewTempFileSinkOption) (*Sink, error) {
	var opt newTmpFileSinkOptions
	for _, o := range opts {
		o(&opt)
	}

	logger := &lumberjack.Logger{
		Filename:   "/tmp/grpcgo_binarylog.bin",
		MaxSize:    1024 * 1024 * 1024 * 1024, // basically infinity; we want to control the rotation on a log entry boundary
		MaxBackups: 3,
	}

	sink := sink.NewBufferedSink(logger)
	return &Sink{
		rotate:      logger.Rotate,
		currentSink: sink,
		sizeLimit:   opt.sizeLimit,
	}, nil
}

func (s *Sink) Write(entry *pb.GrpcLogEntry) error {
	s.Lock()
	defer s.Unlock()

	const headerSize = 4
	entrySizeEstimate := uint64(proto.Size(entry)) + headerSize

	s.currentSize += entrySizeEstimate

	if s.sizeLimit > 0 && s.currentSize > s.sizeLimit {
		log.Println("rotating binary log")
		if s, ok := s.currentSink.(Flusher); ok {
			if err := s.Flush(); err != nil {
				log.Println(err)
			}
		}
		if err := s.rotate(); err != nil {
			log.Println(err)
		}
		s.currentSize = 0
	}
	return s.currentSink.Write(entry)
}

func (s *Sink) Close() error {
	return s.currentSink.Close()
}
