package rotatingbinarylog

import (
	"log"
	"sync"

	"google.golang.org/grpc/binarylog"
	pb "google.golang.org/grpc/binarylog/grpc_binarylog_v1"
	"google.golang.org/protobuf/proto"
)

type Sink struct {
	sync.Mutex

	currentSink binarylog.Sink
	currentSize uint64

	sizeLimit uint64
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

	sink, err := binarylog.NewTempFileSink()
	if err != nil {
		return nil, err
	}
	return &Sink{
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
		if err := s.currentSink.Close(); err != nil {
			log.Println(err)
		}

		log.Println("creating new binarylog file")
		if sink, err := binarylog.NewTempFileSink(); err != nil {
			s.currentSink = nil
			log.Println(err)
		} else {
			s.currentSink = sink
		}
		s.currentSize = 0
	}
	return s.currentSink.Write(entry)
}

func (s *Sink) Close() error {
	return s.currentSink.Close()
}
