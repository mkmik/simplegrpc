package rotatingbinarylog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	pb "google.golang.org/grpc/binarylog/grpc_binarylog_v1"
	"google.golang.org/protobuf/proto"
)

func TestRotatingBinaryLog(t *testing.T) {
	const (
		messageSize = 1024
		numMessages = 1

		dummyDelay = 1000 * time.Millisecond
	)

	tmpfile, err := os.CreateTemp("/tmp", "rotatingbinarylog_")
	require.NoError(t, err)
	//	defer os.RemoveAll(tmpfile.Name())

	fmt.Fprintf(tmpfile, "blah")
	tmpfile.Close()

	logFilesGlob := tmpfile.Name() + "*"

	fileSize := func(filename string) int64 {
		st, err := os.Stat(filename)
		require.NoError(t, err)
		return st.Size()
	}

	snapshot := func(header string) {
		time.Sleep(dummyDelay)
		logFiles, err := filepath.Glob(logFilesGlob)
		require.NoError(t, err)

		sort.Strings(logFiles)
		t.Logf("files: (%s)", header)
		for _, filename := range logFiles {
			t.Logf("  filename=%q size=%d", filename, fileSize(filename))
		}
		t.Log("end files")
	}

	snapshot("before creation")

	sink, err := NewSink(
		WithFilename(tmpfile.Name()),
		WithMaxSize(messageSize*numMessages),
		WithMaxRotations(1),
	)
	require.NoError(t, err)
	snapshot("after creation")

	time.Sleep(dummyDelay)
	logFiles, err := filepath.Glob(logFilesGlob)
	require.NoError(t, err)
	require.Equal(t, 2, len(logFiles), "Log files shyould be rotated upon sink creation")

	entry := pb.GrpcLogEntry{
		Payload: &pb.GrpcLogEntry_Message{Message: &pb.Message{
			Data: make([]byte, messageSize),
		}},
	}
	size := proto.Size(&entry)
	t.Logf("entry size %d\n", size)

	err = sink.Write(&entry)
	require.NoError(t, err)
	snapshot("after first log entry")

	time.Sleep(dummyDelay)
	logFiles, err = filepath.Glob(logFilesGlob)
	require.NoError(t, err)
	require.Equal(t, 2, len(logFiles), "The should be room for one message")

	err = sink.Write(&entry)
	require.NoError(t, err)
	snapshot("after second log entry")

	if true {
		time.Sleep(dummyDelay)
		logFiles, err = filepath.Glob(logFilesGlob)
		require.NoError(t, err)
		if false {
			require.Equal(t, 3, len(logFiles), "The second message should have caused a rotation.")
		}
	}
	//	time.Sleep(1*time.Minute + 2*time.Second)
	time.Sleep(dummyDelay)
	err = sink.Close()
	require.NoError(t, err)

	// The rotation happened after the second log entry was written, but since rotation happens at log entry boundary
	// the second log entry got fully written in the current log file before rotating it out (overflowing the maxSize limit a bit).
	// We should observe that the new current log file is still zero in size.
	snapshot("after closing")

	t.Errorf("TODO")
}
