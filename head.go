package parallel

// head provides a stable, lifetime io.Writer interface for RunFunc - one for each of
// stdout and stderr. It adapts the io.Writer interface of the application to the
// writer interface of “parallel”.
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
