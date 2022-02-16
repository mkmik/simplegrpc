// Package rotatingbinarylog implements a sink for google.golang.org/grpc/grpclog
//
// The sink implements a buffered file writer (based on the builtin implementation from the grpc library, with some minor modifications)
// that can rotate files when they reach a maximum size. It also deletes old log files.
package rotatingbinarylog

import (
	"fmt"
	"sync"

	"github.com/mkmik/simplegrpc/rotatingbinarylog/internal/sink"
	"google.golang.org/grpc/binarylog"
	pb "google.golang.org/grpc/binarylog/grpc_binarylog_v1"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// DefaultFilename is the default filename.
	//
	// We intentionally don't default to a filename that contains the PID so that when
	// the process/container restarts it will GC the old logs, keeping the total log size at bay.
	//
	// If you're not using this package from a container and you may have multiple processes using this
	// feature at the same time, a common technique is to derive the filename from the process ID:
	//
	//     rotatingbinarylog.WithFilename(fmt.Sprintf("/tmp/grpcgo_binarylog_%d.bin", os.Getpid()))
	DefaultFilename = "/tmp/grpcgo_binarylog.bin"
)

var grpclogLogger = grpclog.Component("rotatingbinarylog")

// Sink is a rotating binary log sink you can pass to binarylog.SetSink.
type Sink struct {
	sync.Mutex

	logger *lumberjack.Logger
	sink   binarylog.Sink

	currentSize uint64
	maxSize     uint64
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
		MaxBackups: opt.maxRotations,
	}

	if err := logger.Rotate(); err != nil {
		return nil, fmt.Errorf("initial rotation failed: %w", err)
	}

	return &Sink{
		logger:  logger,
		sink:    sink.NewBufferedSink(logger),
		maxSize: opt.maxSize,
	}, nil
}

type NewSinkOption func(*newSinkOptions)

type newSinkOptions struct {
	filename     string
	maxSize      uint64
	maxRotations int
}

// WithFilename defines the filename for the binary log.
//
// Rotated files will have a filename derived from it,
// for example if the filename is "/tmp/grpcgo_binarylog.bin" the rotated filename will
// "/tmp/grpcgo_binarylog_14097-2022-02-11T12-11-30.976.bin".
func WithFilename(filename string) NewSinkOption {
	return func(opt *newSinkOptions) {
		opt.filename = filename
	}
}

// WithMaxSize defines the maximum size of an individual binary log file.
func WithMaxSize(maxSize uint64) NewSinkOption {
	return func(opt *newSinkOptions) {
		opt.maxSize = maxSize
	}
}

// WithMaxRotations defines how many rotated files to to keep.
func WithMaxRotations(maxRotations int) NewSinkOption {
	return func(opt *newSinkOptions) {
		opt.maxRotations = maxRotations
	}
}

// Our fork of the internal/sink package exposes the Flush method so that we can call it before rotating the file.
// This interface allows us to access that method.
type flusher interface {
	Flush() error
}

func (s *Sink) Write(entry *pb.GrpcLogEntry) error {
	s.Lock()
	defer s.Unlock()

	s.currentSize += uint64(proto.Size(entry)) + sink.HeaderSize

	if s.maxSize > 0 && s.currentSize > s.maxSize {
		grpclogLogger.Warningf("rotating gRPC binary log. size: %d bytes max size: %d bytes", s.currentSize, s.maxSize)

		if s, ok := s.sink.(flusher); ok {
			if err := s.Flush(); err != nil {
				grpclogLogger.Error(err)
				panic(err)
			}
		}
		if err := s.logger.Rotate(); err != nil {
			grpclogLogger.Error(err)
		}
		s.currentSize = 0
	}
	return s.sink.Write(entry)
}

func (s *Sink) Close() error {
	s.Lock()
	defer s.Unlock()

	if s, ok := s.sink.(flusher); ok {
		if err := s.Flush(); err != nil {
			grpclogLogger.Error(err)
		}
	}

	if err := s.sink.Close(); err != nil {
		return fmt.Errorf("closing rotatingbinarylog: %w", err)
	}

	if err := s.logger.Close(); err != nil {
		return fmt.Errorf("closing rotatingbinarylog: %w", err)
	}

	return nil
}
