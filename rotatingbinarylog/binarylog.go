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

const (
	DefaultFilename = "/tmp/grpcgo_binarylog.bin"
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

type NewSinkOption func(*newSinkOptions)

type newSinkOptions struct {
	filename string
	maxSize  uint64
	rotate   int
}

func WithFilename(filename string) NewSinkOption {
	return func(opt *newSinkOptions) {
		opt.filename = filename
	}
}

func WithMaxSize(maxSize uint64) NewSinkOption {
	return func(opt *newSinkOptions) {
		opt.maxSize = maxSize
	}
}

func WithRotate(rotate int) NewSinkOption {
	return func(opt *newSinkOptions) {
		opt.rotate = rotate
	}
}

// NewSink creates a Sink with the provided options.
//
// The Sink implements binarylogger.Sink. Log entries are written to
// a file using buffered IO. Every 60 seconds (currently non configurable) a flush is forced.
//
// When the file exceeds a configurable maximum size it's rotated.
//
// When the number of rotated files exceeds a configurable maximum number of rotated files,
// the oldest files are deleted.
//
// Upon startup, it ensures that the number of old files matches the configured number of
// rotated files; this means that files left over from a previous run are properly reclaimed.
//
// Upon startup, it forces the rotation of the previous active log file.
func NewSink(opts ...NewSinkOption) (*Sink, error) {
	opt := newSinkOptions{
		filename: DefaultFilename,
	}
	for _, o := range opts {
		o(&opt)
	}

	logger := &lumberjack.Logger{
		Filename: opt.filename,
		// We want to control the rotation on a log entry boundary.
		// We will track message size and call Rotate manually when we cross the max size.
		// We cannot set the lumberjack size to infinity so we set to a very large number.
		MaxSize:    1024 * 1024 * 1024 * 1024,
		MaxBackups: opt.rotate,
	}

	if err := logger.Rotate(); err != nil {
		return nil, err
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

	entrySizeEstimate := uint64(proto.Size(entry)) + sink.HeaderSize

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
