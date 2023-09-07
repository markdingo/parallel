package parallel

import (
	"io"
	"sync"
)

type destination int

const (
	toNowhere destination = iota
	toStdout
	toStderr
)

func (d destination) String() string {
	switch d {
	case toNowhere:
		return "toNowhere"
	case toStdout:
		return "toStdout"
	case toStderr:
		return "toStderr"
	}

	return "??destination"
}

type queueState int

const (
	backgroundWithLimit queueState = iota
	backgroundNoLimit
	blocked
	draining
	foreground
)

func (qs queueState) String() string {
	switch qs {
	case backgroundWithLimit:
		return "backgroundWithLimit"
	case backgroundNoLimit:
		return "backgroundNoLimit"
	case blocked:
		return "blocked"
	case draining:
		return "draining"
	case foreground:
		return "foreground"
	}

	return "??queueState"
}

// queue is the writer at the core of the “parallel” package. It decides whether a Write()
// call is written directly downstream because the runner is in foreground mode or queued
// because the runner is in background mode. It also decides whether the caller is blocked
// or gets control back immediately based on [LimitMemoryPerRunner].
//
// The constructor returns two writers - one for stdout and one for stderr, both of which
// share the same queue.
//
// queue has the following states:
//
//   - backgroundWithLimit: Write() data is queued and control returns based on quota
//   - backgroundNoLimit: Write() data is queued and control returns immediately
//   - blocked: Write() is blocked from doing anything
//   - draining: Debug/Ephemeral state never seen by Write()
//   - foreground: Write() calls are sent directly downstream and control returns
//     when the downstream Write() completes
//
// Apart from Write() calls, an independent goroutine (from [Wait]) calls foreground() to
// switch to foreground mode and transfer all buffered output downstream according to
// [OrderStderr]. This call implies the need for some careful locking and sequencing to
// ensure all output ultimately leaves this writer and leaves in the correct order.
type queue struct {
	commonWriter
	where destination
	cq    *commonQueue
}

type commonQueue struct {
	sync.RWMutex // Use common mutex instead of stdout, stderr locks
	state        queueState
	orderStderr  bool
	limit        uint64 // LimitMemoryPerRunner
	out, err     writer

	used  uint64   // LimitMemoryPerRunner
	block chan any // Writers block here in overQuota state
	buf   chunkBuffer
}

// Create two writers which share all state via a commonQueue
func newQueue(orderStderr bool, limit uint64, out, err writer) (stdout, stderr *queue) {
	cq := &commonQueue{state: backgroundWithLimit, orderStderr: orderStderr,
		limit: limit,
		out:   out, err: err,
		block: make(chan any)}

	if cq.limit == 0 {
		cq.state = backgroundNoLimit
	}

	stdout = &queue{where: toStdout, cq: cq} // Create stdout writer
	stderr = &queue{where: toStderr, cq: cq} // and stderr writer

	stdout.setNext(out) // Set downstream writers
	stderr.setNext(err)

	return
}

// Write is cognizant of commonQueue state and protects it from the possibility of
// concurrent Write() calls *and* concurrent foreground() calls. Idiomatic use of the
// mutex isn't possible in this case so we *carefully* manage unlock() calls.
//
// While the queue writer supports concurrent access, when doing so the output results are
// unpredictable as there is no guarantee as to which of multiple goroutine wake first
// after being blocked, nor which one arrives at the channel blocking call first. Even if
// blocked exit could be synchronized with blocked entry, there is no guarantee what the
// scheduler will do the moment these goroutines are given control.
//
// In short, while concurrent access to the queue writer is acceptable and safe, the
// resulting order of output is unpredictable. So, as with concurrent writes to os.Stdout
// and os.Stderr, the application should not expect much in the way of predictable
// outcomes.
func (wtr *queue) Write(p []byte) (n int, err error) {
	wtr.cq.Lock()

	switch wtr.cq.state {
	case backgroundWithLimit:
		if (wtr.cq.used + uint64(len(p))) <= wtr.cq.limit { // Over the limit?
			n, err = wtr.cq.buf.write(wtr.where, p)
			wtr.cq.used += uint64(n)
			wtr.cq.Unlock()
			break
		}

		wtr.cq.state = blocked
		fallthrough // FALLTHRU

	case blocked:
		wtr.cq.Unlock()
		<-wtr.cq.block // Can only come off here when state == foreground
		n, err = wtr.out.Write(p)

	case backgroundNoLimit:
		n, err = wtr.cq.buf.write(wtr.where, p)
		wtr.cq.Unlock()

	case foreground:
		wtr.cq.Unlock()
		n, err = wtr.out.Write(p)

	default:
		panic(wtr.cq.state.String() + " state should not be visible under mutex protection")
	}

	return
}

// Returns total length of queued writes. Concurrency safe.
func (cq *commonQueue) len() (outLen, errLen int) {
	cq.Lock()
	defer cq.Unlock()

	for _, b := range cq.buf.chunks {
		switch b.where {
		case toStdout:
			outLen += len(b.data)
		case toStderr:
			errLen += len(b.data)
		}
	}

	return
}

func (wtr *queue) close() {
	wtr.foreground()
	wtr.out.close() // Pass it on
}

// Switch writer to foreground. That means draining the chunkBuffer. foreground is
// purposely idempotent to make life easier for Group handling and in the unlikely event
// that foreground is accidentally called for both writers.
//
// foreground does not return until all buffered chunks have been sent downstream and
// before blocked writers are unblocked. It's possible that this call could take a long
// time to finish, depending on what downstream is doing, but there is no need to return
// control in a hurry as [Group.Wait] cannot do anything useful until this transition has
// completed anyway. IOWs, there is no good reason to start a separate goroutine for this.
func (wtr *queue) foreground() {
	wtr.cq.Lock()
	defer wtr.cq.Unlock()

	if wtr.cq.state == foreground {
		return
	}

	wtr.cq.state = draining // This ephemeral state should never be visible inside the mutex
	wtr.cq.buf.drain(wtr.cq.orderStderr, wtr.cq.out, wtr.cq.err)
	wtr.cq.state = foreground
	close(wtr.cq.block) // Free up all blocked Writer() callers
}

// chunk contains the data for a single Write call
type chunk struct {
	where destination
	data  []byte
}

// chunkBuffer contains all Write() data in arrival order. It provides the ability to
// transfer the writes in the same order by way of iterating thru getChunks()
//
// All callers to chunkBuffer must provide concurrency protection.
type chunkBuffer struct {
	chunks []chunk
}

// write appends the supplied bytes to the chunkBuffer. It is normally called as a
// consequence of a writer receiving a Write() call. Write copies the underlying data as
// it is needed long after control returns to the caller who might otherwise assume that
// the data is no longer needed or immutable.
func (buf *chunkBuffer) write(where destination, p []byte) (n int, err error) {
	b := chunk{where: where, data: make([]byte, len(p))}
	copy(b.data, p)
	buf.chunks = append(buf.chunks, b)

	return len(p), nil
}

// Transfer all chunks to downstream writers then release chunks to the GC.
func (buf *chunkBuffer) drain(orderStderr bool, out, err io.Writer) {
	if orderStderr {
		buf.transfer(out, nil)
		buf.transfer(nil, err)
	} else {
		buf.transfer(out, err)
	}
	buf.chunks = []chunk{} // Release to GC and empty slice
}

// transfer all chunks to the downstream writers - if present. Caller is responsible for
// clearing the chunks so that they are not written more than once. If a downstream Write
// fails the transfer stops and the error is returned. As it stands, current callers
// ignore i/o errors as there is no mechanism to pass these errors back to Write() caller
// as they are long gone.
func (buf *chunkBuffer) transfer(out, err io.Writer) error {
	for _, b := range buf.chunks {
		if b.where == toStdout && out != nil {
			_, e := out.Write(b.data)
			if e != nil {
				return e
			}
			continue
		}
		if b.where == toStderr && err != nil {
			_, e := err.Write(b.data)
			if e != nil {
				return e
			}
			continue
		}
	}

	return nil
}
