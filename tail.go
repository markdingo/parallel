package parallel

import (
	"io"
	"sync"
)

// tail provides a transition from a writer to an io.Writer which normally interfaces to
// Group.stdout and Group.stderr which in turn is normally os.Stdout and os.Stderr. Tail
// always represents the last writer in a pipeline which means it never has a "next" so
// getting, setting and closing are all no-ops.
//
// Most importantly, tail protects the Group output writers from concurrent access by all
// runners within the Group.
type tail struct {
	out      io.Writer
	outputMu *sync.Mutex
}

func newTail(out io.Writer, outputMu *sync.Mutex) *tail {
	return &tail{out: out, outputMu: outputMu}
}

func (wtr *tail) getNext() writer {
	return nil
}

func (wtr *tail) setNext(out writer) {
}

func (wtr *tail) Write(p []byte) (n int, err error) {
	wtr.outputMu.Lock()
	defer wtr.outputMu.Unlock()
	return wtr.out.Write(p)
}

func (wtr *tail) close() {}
