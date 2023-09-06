package parallel

// head provides a stable, lifetime io.Writer interface for RunFunc for both stdout and
// stderr and it provides the Write() pathway thru Pipeline via the commonWriter.out
// io.Writer.
type head struct {
	commonWriter
}

func newHead(out writer) *head {
	wtr := &head{}
	wtr.setNext(out)

	return wtr
}

func (wtr *head) Write(p []byte) (n int, err error) {
	return wtr.out.Write(p)
}

func (wtr *head) close() {
	wtr.out.close() // Pass it on
}
