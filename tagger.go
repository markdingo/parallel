package parallel

import (
	"bytes"
	"sync"
)

// tagger is a writer which prepends the tag to each line terminated with "\n" and writes
// it to the next writer in the pipeline. No data is buffered in this writer, only state
// information pertaining to tag insertion is tracked.
type tagger struct {
	mu sync.Mutex
	commonWriter
	tag        []byte
	tagPending bool
}

func newTagger(out writer, tag []byte) *tagger {
	wtr := &tagger{tag: tag, tagPending: true}
	wtr.setNext(out)

	return wtr
}

var nl = []byte{'\n'}

// Write prepends tag to each output line. The tag is prepended as soon as the line is
// known to exist, even if it does not yet have a trailing \n.
func (wtr *tagger) Write(p []byte) (n int, err error) {
	if len(p) == 0 { // Make sure len(lines) > 0
		return 0, nil
	}

	if len(wtr.tag) == 0 { // Do nothing if there's no tag
		return wtr.out.Write(p)
	}

	wtr.mu.Lock() // Protect our tagPending state
	defer wtr.mu.Unlock()

	lines := bytes.Split(p, nl)
	for ix := 0; ix < len(lines)-1; ix++ { // Process allbut the last line
		if wtr.tagPending {
			_, e := wtr.out.Write(wtr.tag)
			if e != nil && err == nil { // First error is returned
				err = e
			}
		}
		wtr.tagPending = true
		ln := lines[ix]
		b, e := wtr.out.Write(ln)
		if e != nil && err == nil { // First error is returned
			err = e
		} else { // Otherwise accumulate input bytes written
			n += b
		}

		_, e = wtr.out.Write(nl)
		if e != nil && err == nil { // First error is returned
			err = e
		} else { // Otherwise accumulate input bytes written
			n++
		}
	}

	// If the last line is not empty that means it is a line of data without a
	// trailing "\n". In this case, the tag and line are written and no subsequent tag
	// is pending.
	//
	// If the last line *is* empty, it means that the last line of data had a trailing
	// "\n" and thus a tag is pending for the next inbound Write() call - if it ever
	// comes.

	ln := lines[len(lines)-1]
	if len(ln) > 0 {
		if wtr.tagPending {
			_, e := wtr.out.Write(wtr.tag)
			if e != nil && err == nil { // First error is returned
				err = e
			}
		}
		b, e := wtr.out.Write(ln)
		if e != nil && err == nil { // First error is returned
			err = e
		} else { // Otherwise accumulate input bytes written
			n += b
		}
		wtr.tagPending = false
	} else {
		wtr.tagPending = true
	}

	return
}

func (wtr *tagger) close() {
	wtr.out.close() // pass it on
}
