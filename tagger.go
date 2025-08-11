package parallel

import (
	"bytes"
	"sync"
)

// tagger is a writer which prepends the tag string to each line terminated with "\n" and
// writes it to the next writer in the pipeline. No data is buffered in this writer, only
// state information pertaining to tag insertion is tracked.
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

// Write prepends tag to each output line. The tag is prepended as soon as a non-empty
// line is known to exist, even if it does not yet have a trailing "\n".
//
// The returned byte count is complicated. First off, io.Writer.Write makes it clear that
// this value is valid even when err != nil. That's kinda unusual for a go API. Second
// off, tagger occassionally writes more bytes than it's given by virtue of having to
// write the tag data, but we still want to maintain the illusion that tagger is merely
// writing what the caller provides.
//
// Finally, tagger functionality implies that one inbound Write() can result in multiple
// outbound Write() calls so multiple error returns from the outbound Write() are possible
// and need to be coalesced. Our chosen solution is to return the first error as
// subsequent errors may have masked the initial error.
//
// Thus, all in all, error returns are a bit hit and miss and the caller really can't
// assume much about anything, but we do what we can to make the return values as useful
// as possible.
func (wtr *tagger) Write(p []byte) (n int, err error) {
	if len(p) == 0 { // Make sure len(lines) > 0
		return 0, nil // W0: Zero len data
	}

	if len(wtr.tag) == 0 { // Pass straight thru if there's no tag
		return wtr.out.Write(p) // W1: Passthru
	}

	wtr.mu.Lock() // Protect our local writer state
	defer wtr.mu.Unlock()

	lines := bytes.Split(p, nl)
	for ix := range len(lines) - 1 { // Process allbut the last line
		if wtr.tagPending {
			_, e := wtr.out.Write(wtr.tag) // W2: Bytes not returned for tag
			if e != nil && err == nil {    // but first error is always returned
				err = e
			}
		}
		wtr.tagPending = true // Always true for second and subsequent lines

		ln := lines[ix]
		b, e := wtr.out.Write(ln)   // W3: Line of data
		if e != nil && err == nil { // First error is always returned
			err = e
		}
		n += b // Bytes written is always returned for user data

		b, e = wtr.out.Write(nl)    // W4: NL
		if e != nil && err == nil { // First error is always returned
			err = e
		}
		n += b // Bytes written is always returned for user data
	}

	// If the last line is not empty that means it is a line of data without a
	// trailing "\n" due to bytes.Split(). In this case the tag and line are written
	// and tagPending is set false.
	//
	// If the last line *is* empty, it means that the last line of data had a trailing
	// "\n" and thus tagPending is set for the next inbound Write() call - if it ever
	// comes.

	ln := lines[len(lines)-1]
	if len(ln) > 0 { // If last line is not empty
		if wtr.tagPending {
			_, e := wtr.out.Write(wtr.tag) // W5: Bytes not returned for tag
			if e != nil && err == nil {    // but first error is always returned
				err = e
			}
		}
		b, e := wtr.out.Write(ln)   // W6: Line of data
		if e != nil && err == nil { // First error is returned
			err = e
		}
		n += b // Bytes written is always returned for user data
		wtr.tagPending = false
	} else {
		wtr.tagPending = true
	}

	return
}

func (wtr *tagger) close() {
	wtr.out.close() // pass it on
}
