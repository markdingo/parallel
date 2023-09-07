package parallel

import (
	"bytes"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroupState(t *testing.T) {
	grp, err := NewGroup()
	if err != nil {
		t.Fatal("Unexpected setup error", err)
	}

	didPanic := tgCallRun(grp)
	if didPanic {
		t.Error("Run with Add should not cause a panic")
	}
	didPanic = tgCallWait(grp)
	if didPanic {
		t.Error("Wait after Run should not cause a panic")
	}

	// All should now panic
	didPanic = tgCallRun(grp)
	if !didPanic {
		t.Error("Run after Wait should panic")
	}
	didPanic = tgCallWait(grp)
	if !didPanic {
		t.Error("Wait after Wait should panic")
	}
}

func tgCallRun(grp *Group) (didPanic bool) {
	defer func() {
		didPanic = recover() != nil
	}()
	grp.Run()

	return
}

func tgCallWait(grp *Group) (didPanic bool) {
	defer func() {
		didPanic = recover() != nil
	}()
	grp.Wait()

	return
}

type testRunFunc struct {
	delay  time.Duration // Lazy sequencing. Delay before writing.
	chunks []chunk
}

func (trf *testRunFunc) addChunk(where destination, s string) {
	b := chunk{where: where, data: make([]byte, len(s))}
	b.data = []byte(s)
	trf.chunks = append(trf.chunks, b)
}

func (trf *testRunFunc) run(out, err io.Writer) {
	time.Sleep(trf.delay)
	for _, chunk := range trf.chunks {
		if chunk.where == toStdout {
			out.Write(chunk.data)
		} else {
			err.Write(chunk.data)
		}
	}
}

func TestGroupOrderRunnersTrue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	grp, err := NewGroup(WithStdout(&stdout), WithStderr(&stderr),
		OrderRunners(true))
	if err != nil {
		t.Fatal("Unexpected setup error", err)
	}

	f1 := &testRunFunc{delay: time.Millisecond * 100}
	f1.addChunk(toStdout, "f1: Single Out Line\n")
	f1.addChunk(toStderr, "f1: Single Err Line\n")

	f2 := &testRunFunc{}
	f2.addChunk(toStdout, "f2: Single Out Line\n")
	f2.addChunk(toStderr, "f2: Single Err Line\n")

	grp.Add("", "", f1.run)
	grp.Add("", "", f2.run) // f2 should finish first due to f1.delay

	grp.Run()
	grp.Wait()

	actual := stdout.String()
	expect := "f1: Single Out Line\nf2: Single Out Line\n"
	if actual != expect {
		t.Error("OrderRunners(true) did not honour stdout.\nExpect:\n",
			expect, "\nActual\n", actual)
	}
	actual = stderr.String()
	expect = "f1: Single Err Line\nf2: Single Err Line\n"
	if actual != expect {
		t.Error("OrderRunners(true) did not honour stderr.\nExpect:\n",
			expect, "\nActual\n", actual)
	}
}

// Same as above but with OrderRunners(false) which should result in f2 output coming
// first.
func TestGroupOrderRunnersFalse(t *testing.T) {
	var stdout, stderr bytes.Buffer
	grp, err := NewGroup(WithStdout(&stdout), WithStderr(&stderr), OrderRunners(false))
	if err != nil {
		t.Fatal("Unexpected setup error", err)
	}

	f1 := &testRunFunc{delay: time.Millisecond * 100}
	f1.addChunk(toStdout, "f1: Single Out Line\n")
	f1.addChunk(toStderr, "f1: Single Err Line\n")

	f2 := &testRunFunc{}
	f2.addChunk(toStdout, "f2: Single Out Line\n")
	f2.addChunk(toStderr, "f2: Single Err Line\n")

	grp.Add("", "", f1.run)
	grp.Add("", "", f2.run) // f2 should finish first due to no delays

	grp.Run()
	grp.Wait()

	actual := stdout.String()
	expect := "f2: Single Out Line\nf1: Single Out Line\n"
	if actual != expect {
		t.Error("OrderRunners(true) did not honour stdout.\nExpect:\n",
			expect, "\nActual\n", actual)
	}
	actual = stderr.String()
	expect = "f2: Single Err Line\nf1: Single Err Line\n"
	if actual != expect {
		t.Error("OrderRunners(true) did not honour stderr.\nExpect:\n",
			expect, "\nActual\n", actual)
	}
}

func TestGroupOrderStderr(t *testing.T) {
	var buf bytes.Buffer
	grp, err := NewGroup(WithStdout(&buf), WithStderr(&buf),
		OrderRunners(true), OrderStderr(true))
	if err != nil {
		t.Fatal("Unexpected setup error", err)
	}

	f1 := &testRunFunc{delay: time.Millisecond * 100}
	f1.addChunk(toStdout, "f1: First Out Line\n")
	f1.addChunk(toStderr, "f1: Single Err Line\n") // This should come after the next chunk
	f1.addChunk(toStdout, "f1: Last Out Line\n")

	f2 := &testRunFunc{}
	f2.addChunk(toStdout, "f2: First Out Line\n")
	f2.addChunk(toStderr, "f2: Single Err Line\n") // This should come after the next chunk
	f2.addChunk(toStdout, "f2: Last Out Line\n")

	grp.Add("", "", f1.run)
	grp.Add("", "", f2.run)

	grp.Run()
	grp.Wait()

	actual := buf.String()
	expect := "f1: First Out Line\nf1: Last Out Line\nf1: Single Err Line\n"
	expect += "f2: First Out Line\nf2: Last Out Line\nf2: Single Err Line\n"
	if actual != expect {
		t.Error("OrderStderr(true) did not honour stdout.\nExpect:\n",
			expect, "\nActual\n", actual)
	}
}

func TestGroupPassthru(t *testing.T) {
	var buf bytes.Buffer
	grp, err := NewGroup(WithStdout(&buf), WithStderr(&buf), Passthru(true), OrderRunners(false))
	if err != nil {
		t.Fatal("Unexpected setup error", err)
	}

	f1 := &testRunFunc{delay: time.Millisecond * 100}
	f1.addChunk(toStdout, "f1: First Out Line\n")
	f1.addChunk(toStderr, "f1: Single Err Line\n")
	f1.addChunk(toStdout, "f1: Last Out Line\n")

	f2 := &testRunFunc{}
	f2.addChunk(toStdout, "f2: First Out Line\n")
	f2.addChunk(toStderr, "f2: Single Err Line\n")
	f2.addChunk(toStdout, "f2: Last Out Line\n")

	grp.Add("", "", f1.run)
	grp.Add("", "", f2.run)

	grp.Run()
	grp.Wait()

	actual := buf.String()
	expect := "f2: First Out Line\nf2: Single Err Line\nf2: Last Out Line\n"
	expect += "f1: First Out Line\nf1: Single Err Line\nf1: Last Out Line\n"
	if actual != expect {
		t.Error("OrderStderr(true) did not honour stdout.\nExpect:\n",
			expect, "\nActual\n", actual)
	}
}

// Seps and tags while we're at it
func TestGroupSeparators(t *testing.T) {
	var stdout, stderr bytes.Buffer
	grp, err := NewGroup(WithStdout(&stdout), WithStderr(&stderr),
		OrderRunners(true), OrderStderr(true),
		WithStdoutSeparator("OUT\n"), WithStderrSeparator("ERR\n"))
	if err != nil {
		t.Fatal("Unexpected setup error", err)
	}

	f1 := &testRunFunc{}
	f1.addChunk(toStdout, "f1: First Out Line\n")
	f1.addChunk(toStderr, "f1: Single Err Line\n")
	f1.addChunk(toStdout, "f1: Last Out Line\n")

	f2 := &testRunFunc{}
	f2.addChunk(toStdout, "f2: First Out Line\n")
	f2.addChunk(toStderr, "f2: Single Err Line\n")
	f2.addChunk(toStdout, "f2: Last Out Line\n")

	grp.Add("1o: ", "1e: ", f1.run) // Ask parallel to prepend tags
	grp.Add("2o: ", "2e: ", f2.run)

	grp.Run()
	grp.Wait()

	actual := stdout.String()
	expect := "1o: f1: First Out Line\n1o: f1: Last Out Line\n"
	expect += "OUT\n"
	expect += "2o: f2: First Out Line\n2o: f2: Last Out Line\n"
	if actual != expect {
		t.Error("GroupSeparator+Tags did not honour stdout.\nExpect:\n",
			expect, "\nActual\n", actual)
	}
	actual = stderr.String()
	expect = "1e: f1: Single Err Line\n"
	expect += "ERR\n"
	expect += "2e: f2: Single Err Line\n"
	if actual != expect {
		t.Error("GroupSeparator+Tags did not honour stderr.\nExpect:\n",
			expect, "\nActual\n", actual)
	}
}

type testMemoryRunner struct {
	id      int
	howMany int
	line    string
	start   chan any
	done    chan any
}

func (tqr *testMemoryRunner) run(stdout, stderr io.Writer) {
	<-tqr.start
	for tqrCount := 0; tqrCount < tqr.howMany; tqrCount++ {
		n, _ := stdout.Write([]byte(tqr.line))
		testTBWritten.Add(int32(n))
	}
	tqr.done <- true
	tqr.done <- true
}

var testTBWritten atomic.Int32

// Most errors are treated as fatal because there is channel synchronization used to
// progress runners and once there is an error, the state of the channels is unknown and
// thus further results are likely unpredictable anyway.
func TestGroupLimitMemoryPerRunner(t *testing.T) {
	const limit = 100
	var stdout, stderr bytes.Buffer
	grp, err := NewGroup(WithStdout(&stdout), WithStderr(&stderr),
		OrderRunners(true), LimitMemoryPerRunner(limit), LimitActiveRunners(2))
	if err != nil {
		t.Fatal("Did not expect NewGroup error", err)
	}

	if grp.limitMemory != limit {
		t.Fatal("Memory limit should have be set to", limit, "not", grp.limitMemory)
	}

	tqr1 := &testMemoryRunner{id: 1, howMany: 10, line: "19 bytes + NLxxxxxx\n",
		start: make(chan any), done: make(chan any)} // 200 bytes
	tqr2 := &testMemoryRunner{id: 2, howMany: 20, line: "19 bytes + NLyyyyyy\n",
		start: make(chan any), done: make(chan any)} // 400 bytes
	grp.Add("one\t", "", tqr1.run) // Tags should impact memory limits
	grp.Add("two\t", "", tqr2.run) // as they are applied *after* queue writer
	grp.Run()

	go grp.Wait() // Need to have Wait() running to progress runners
	tqr1.start <- true // Release tqr1
	<-tqr1.done        // wait until it's done

	// Since tqr1 should be in fg, no memory limits apply and thus it can write as
	// much as it wants beyond the limit value.

	tb := int(testTBWritten.Load())
	if tb != tqr1.howMany*len(tqr1.line) {
		t.Fatal("Foreground Runner should have successfully written to stdout")
	}

	// At this stage, as far as parallel is concerned, tqr1 is still running, so tqr2
	// cannot be switched to fg, ergo, memory limits will apply to tqr2
	tqr2.start <- true // Release tqr2

	// Wait for tb to stop changing which implies that tqr2 has stopped writing due to
	// reaching memory limits.
	firstTB := tb
	lastTB := tb
	for ix := 0; ix < 10; ix++ {
		time.Sleep(time.Millisecond * 100)
		tb = int(testTBWritten.Load())
		if tb == lastTB {
			break
		}
		lastTB = tb
	}

	if lastTB != firstTB+limit {
		t.Fatal("Expected tqr2 to write", firstTB+limit, "not", lastTB)
	}

	// Releasing tqr1 should switch tqr2 to foreground so it should complete
	<-tqr1.done
	time.Sleep(time.Millisecond * 200)
	tb = int(testTBWritten.Load())
	if tb == lastTB {
		t.Error("tqr2 stalled")
	}
}

func TestGroupErrors(t *testing.T) {
	_, err := NewGroup(WithStdout(nil))
	if err == nil {
		t.Error("Expected error return from WithStdout(nil)")
	}
	_, err = NewGroup(WithStderr(nil))
	if err == nil {
		t.Error("Expected error return from WithStderr(nil)")
	}
}
