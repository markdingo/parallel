package parallel

import (
	"container/list"
	"sync"
)

// runner manages the life-cycle and pipeline of each RunFunc.
type runner struct {
	rFunc          RunFunc // Function started as a goroutine by Run()
	outTag, errTag []byte  // Prepended to each output line

	sync.RWMutex          // Protects everything below
	stdout, stderr writer // Immutable "head" supplied to Run()
	queue          *queue // Remember queue so we can flush() it
	canClose       bool   // If Wait() has read this runner from completed channel
}

// newRunner constructs a skeletal *runner with no pipeline.
func newRunner(outTag, errTag string, rFunc RunFunc) *runner {
	return &runner{outTag: []byte(outTag), errTag: []byte(errTag), rFunc: rFunc}
}

// The queue pipeline consists of head, queue tagger, tail and Group.stdout/Group.stderr
// built in reverse order as it's stored as a singly linked list. A queue pipeline starts
// out in background mode.
func (rnr *runner) buildQueuePipeline(grp *Group) {
	var stdout, stderr writer
	stdout = newTail(grp.stdout, &grp.outputMu)
	stderr = newTail(grp.stderr, &grp.outputMu)

	// Tagging is optional, so leave them out if not set
	if len(rnr.outTag) > 0 {
		stdout = newTagger(stdout, rnr.outTag)
	}
	if len(rnr.errTag) > 0 {
		stderr = newTagger(stderr, rnr.errTag)
	}

	// Queue creates two writers which share an output buffer for sequencing and
	// background storage purposes. We remember one of the Queue writers so that we
	// can switch it to foreground at a later time.

	rnr.queue, stderr = newQueue(grp.orderStderr, grp.limitMemory, stdout, stderr)
	stdout = rnr.queue

	stdout = newHead(stdout)
	stderr = newHead(stderr)

	rnr.stdout = stdout
	rnr.stderr = stderr
}

// The passthru pipeline consists of head, tail and Group.stdout/Group.stderr which
// eliminates all writers with state but still retains concurrency protection for the
// Group io.Writers. So not strictly a true passthru, but as close as we can get while
// still protecting Group outputs.
func (rnr *runner) buildPassthruPipeline(grp *Group) {
	rnr.stdout = newHead(newTail(grp.stdout, &grp.outputMu))
	rnr.stderr = newHead(newTail(grp.stderr, &grp.outputMu))
}

// switchToForeground is called when the runner is allowed to write directly to the Group
// io.Writers. The queue writer manages the transition by releasing its queue of pending
// writes and unblocking any blocked callers.
func (rnr *runner) switchToForeground() {
	rnr.queue.foreground()
}

// run the RunFunc and notify completion to [Group.Wait]. This function is the RunFunc
// goroutine so nothing is stalled by potentially blocking on the completion channel.
func (rnr *runner) run(e *list.Element, completed chan *list.Element) {
	rnr.rFunc(rnr.stdout, rnr.stderr)
	completed <- e
}

// Flush all pending output
func (rnr *runner) close() {
	rnr.stdout.close()
	rnr.stderr.close()
}
