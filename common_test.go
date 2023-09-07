package parallel

import (
	"bytes"
	"sync"
)

// A writer which collects output to see what the Pipeline has let thru.
type testBufWriter struct {
	id string
	mu sync.Mutex
	commonWriter
	buf bytes.Buffer
}

func (tbw *testBufWriter) Write(p []byte) (n int, err error) {
	tbw.mu.Lock()
	defer tbw.mu.Unlock()

	return tbw.buf.Write(p)
}

func (tbw *testBufWriter) String() string {
	return tbw.buf.String()
}

func (tbw *testBufWriter) close() {}

func (tbw *testBufWriter) Len() int {
	tbw.mu.Lock()
	defer tbw.mu.Unlock()

	return tbw.buf.Len()
}

// A Writer which can be configured to return a truncated Write() and/or an error.
type testTruncateWriter struct {
	len int
	err error
}

func (ttw *testTruncateWriter) Write(p []byte) (n int, err error) {
	return ttw.len, ttw.err
}
