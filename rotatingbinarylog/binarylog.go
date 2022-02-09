package rotatingbinarylog

import (
	"sync"

	"github.com/mkmik/simplegrpc/rotatingbinarylog/internal/sink"
	"google.golang.org/grpc/binarylog"
	pb "google.golang.org/grpc/binarylog/grpc_binarylog_v1"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
	"gopkg.in/natefinch/lumberjack.v2"
)

var grpclogLogger = grpclog.Component("rotatingbinarylog")

type Sink struct {
	sync.Mutex

	rotate func() error
	sink   binarylog.Sink

	currentSize uint64
	maxSize     uint64
}

type Flusher interface {
	Flush() error
}

type NewTempFileSinkOption func(*newTmpFileSinkOptions)

type newTmpFileSinkOptions struct {
	maxSize uint64
	rotate  int
}

func WithMaxSize(maxSize uint64) NewTempFileSinkOption {
	return func(opt *newTmpFileSinkOptions) {
		opt.maxSize = maxSize
	}
}

func WithRotate(rotate int) NewTempFileSinkOption {
	return func(opt *newTmpFileSinkOptions) {
		opt.rotate = rotate
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
		MaxBackups: opt.rotate,
	}

	sink := sink.NewBufferedSink(logger)
	return &Sink{
		rotate:  logger.Rotate,
		sink:    sink,
		maxSize: opt.maxSize,
	}, nil
}

func (s *Sink) Write(entry *pb.GrpcLogEntry) error {
	s.Lock()
	defer s.Unlock()

	const headerSize = 4
	entrySizeEstimate := uint64(proto.Size(entry)) + headerSize

	s.currentSize += entrySizeEstimate

	if s.maxSize > 0 && s.currentSize > s.maxSize {
		grpclogLogger.Warningf("rotating gRPC binary log. size: %d bytes max size: %d bytes", s.currentSize, s.maxSize)
		if s, ok := s.sink.(Flusher); ok {
			if err := s.Flush(); err != nil {
				grpclogLogger.Error(err)
			}
		}
		if err := s.rotate(); err != nil {
			grpclogLogger.Error(err)
		}
		s.currentSize = 0
	}
	return s.sink.Write(entry)
}

func (s *Sink) Close() error {
	return s.sink.Close()
}
