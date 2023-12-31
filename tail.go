package parallel

import (
	"io"
	"sync"
)

// tail adapts our writer interface to an io.Writer interface which normally points to
// Group.stdout and Group.stderr which in turn normally points to os.Stdout and
// os.Stderr. Tail always represents the last writer in a pipeline which means it never
// has a "next" writer so getting, setting and closing functions are all no-ops.
//
// Most importantly, tail protects the Group output writers from concurrent access by all
// runners within the Group via a group-wide mutex.
type tail struct {
	out      io.Writer
	outputMu *sync.Mutex
}

func newTail(out io.Writer, outputMu *sync.Mutex) *tail {
	return &tail{out: out, outputMu: outputMu}
}

func (wtr *tail) getNext() writer { return nil }
func (wtr *tail) setNext(writer)  {}
func (wtr *tail) close()          {}

func (wtr *tail) Write(p []byte) (n int, err error) {
	wtr.outputMu.Lock()
	defer wtr.outputMu.Unlock()
	return wtr.out.Write(p)
}
