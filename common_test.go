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

type testTTWresult struct {
	desc string
	len  int // 0: return 0, -1: return len(p), otherwise use len
	err  error
}

// A Writer which can be configured to return a truncated Write() and/or an error. Each
// call to Write pops a result off the stack and returns the results. If the stack is
// empty the Write returns success. All successful writes are stored in buf.
type testTruncateWriter struct {
	index   int
	results []testTTWresult
	buf     bytes.Buffer
}

func (ttw *testTruncateWriter) String() string {
	return ttw.buf.String()
}

// Place this result at the bottom of the stack
func (ttw *testTruncateWriter) append(desc string, len int, err error) {
	ttw.results = append(ttw.results, testTTWresult{desc, len, err})
}

// Pop the next results and return them to caller. A zero length returns zero, a positive
// length means return that value and -1 means return the length of the slice.
func (ttw *testTruncateWriter) Write(p []byte) (n int, err error) {
	ttw.index++
	if len(ttw.results) == 0 { // If stack is empty, return complete success
		return ttw.buf.Write(p)
	}

	r := ttw.results[0] // Pop off stack
	ttw.results = ttw.results[1:]
	err = r.err
	switch r.len {
	case 0:
		n = 0
	case -1:
		n = len(p)
	default:
		n = r.len
		if n > len(p) {
			n = len(p)
		}
	}

	if n > 0 {
		ttw.buf.Write(p[:n])
	}

	return
}

func (ttw *testTruncateWriter) close() {
}

func (ttw *testTruncateWriter) getNext() writer {
	return nil
}

func (ttw *testTruncateWriter) setNext(w writer) {
}
