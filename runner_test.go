package parallel

import (
	"fmt"
	"slices"
	"testing"
)

// Test that runner builds the correct pipeline based on config settings
func TestRunnerBuildQueue(t *testing.T) {
	grp, err := NewGroup()
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	rnr := newRunner("", "", nil)
	rnr.buildQueuePipeline(grp)

	ow, ew := testGetWriters(rnr)
	expect := []string{"*parallel.head", "*parallel.queue", "*parallel.tail"}
	if slices.Compare(expect, ow) != 0 {
		t.Error("Queue Pipeline stdout mismatch got", ow, "expect", expect)
	}
	if slices.Compare(expect, ew) != 0 {
		t.Error("Queue Pipeline stderr mismatch got", ew, "expect", expect)
	}

	// Tags
	grp, err = NewGroup()
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	rnr = newRunner("out", "err", nil)
	rnr.buildQueuePipeline(grp)

	ow, ew = testGetWriters(rnr)
	expect = []string{"*parallel.head", "*parallel.queue", "*parallel.tagger", "*parallel.tail"}
	if slices.Compare(expect, ow) != 0 {
		t.Error("Queue Pipeline stdout mismatch got", ow, "expect", expect)
	}
	if slices.Compare(expect, ew) != 0 {
		t.Error("Queue Pipeline stderr mismatch got", ew, "expect", expect)
	}
}

func testGetWriters(rnr *runner) (outWriters, errWriters []string) {
	for o := rnr.stdout; o != nil; o = o.getNext() {
		outWriters = append(outWriters, fmt.Sprintf("%T", o))
	}

	for e := rnr.stdout; e != nil; e = e.getNext() {
		errWriters = append(errWriters, fmt.Sprintf("%T", e))
	}

	return
}
