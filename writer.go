package parallel

import (
	"io"
)

// A "pipeline" as it is referred to in this package, consists of a list of writers, much
// like a Unix pipeline. A call to writer.Write() passes down the pipeline from writer to
// writer until it exits back into the Group io.Writers. Write() calls are not necessarily
// synchronous however as writers such as “queue” can buffer output for later forwarding
// down the pipeline.
type writer interface {
	// These are usually implemented by commonWriter
	getNext() writer
	setNext(writer)

	// These are specializations of each writer
	io.Writer
	close()
}

// Most writers uses commonWriter as a base for their implementation. If a writer has
// local state it needs to protect it with its own a local mutex. Nothing in commonWriter
// is modified after initial construction so it has no concurrency controls.
type commonWriter struct {
	out writer
}

func (wtr *commonWriter) getNext() (out writer) {
	return wtr.out
}

func (wtr *commonWriter) setNext(out writer) {
	wtr.out = out
}
