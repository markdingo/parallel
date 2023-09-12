package parallel

import (
	"errors"
	"testing"
	"time"
)

// Test that the queue writer does indeed queue all data as stored as written
func TestQueueBackground(t *testing.T) {
	var outBuf, errBuf testBufWriter
	outQ, errQ := newQueue(false, 100, &outBuf, &errBuf)
	cq := outQ.cq

	outQ.Write([]byte{'a', 'b', 'c'})
	outQ.Write([]byte{'1', '2', '3', '4'})
	outQ.Write([]byte{'y', 'z'})

	ol, el := cq.len()
	if (ol + el) != 9 {
		t.Error("Queue should be in background mode with all output queue, not", el, el)
	}

	if cq.used != 9 {
		t.Error("Used should be 9, not", cq.used)
	}

	errQ.Write([]byte{'A', 'B', 'C'})
	errQ.Write([]byte{'1', '2', '3', '4'})
	errQ.Write([]byte{'Y', 'Z'})
	ol, el = cq.len()
	if (ol != 9) || (el != 9) {
		t.Error("Queued should be 9+9, not", ol, el)
	}

	if cq.used != 18 {
		t.Error("Used should be 18, not", cq.used)
	}

	outQ.foreground()
	errQ.foreground()

	expect := "abc1234yz"
	results := outBuf.String()
	if results != expect {
		t.Error("Data stream corrupted in pipeline", results, "!=", expect)
	}

	expect = "ABC1234YZ"
	results = errBuf.String()
	if results != expect {
		t.Error("Data stream corrupted in pipeline", results, "!=", expect)
	}
}

// Test that writes to both output streams is stored in stream order
func TestQueueOrderStderr(t *testing.T) {
	ob := &testBufWriter{}
	outQ, errQ := newQueue(false, 0, ob, ob)
	outQ.Write([]byte("out a<<"))
	errQ.Write([]byte("err a<<"))
	errQ.Write([]byte("err b<<"))
	outQ.Write([]byte("out b<<"))
	outQ.Write([]byte("out c<<"))
	errQ.Write([]byte("err c<<"))
	outQ.foreground()
	exp := "out a<<err a<<err b<<out b<<out c<<err c<<" // Should be in write order
	got := ob.String()
	if exp != got {
		t.Error("OrderStderr(false) expected\n", exp, "\ngot\n", got)
	}

	ob = &testBufWriter{}
	outQ, errQ = newQueue(true, 0, ob, ob)
	outQ.Write([]byte("out a<<"))
	errQ.Write([]byte("err a<<"))
	errQ.Write([]byte("err b<<"))
	outQ.Write([]byte("out b<<"))
	outQ.Write([]byte("out c<<"))
	errQ.Write([]byte("err c<<"))
	outQ.foreground()
	exp = "out a<<out b<<out c<<err a<<err b<<err c<<"
	got = ob.String()
	if exp != got {
		t.Error("OrderStderr(true) expected\n", exp, "\ngot\n", got)
	}
}

type tqbClient struct {
	line <-chan string
	wtr  writer
}

func (c *tqbClient) run() {
	for ln := range c.line {
		c.wtr.Write([]byte(ln))
	}
}

// Test that queue blocks on Write() once limits have been reached
func TestQueueBlock(t *testing.T) {
	ob := &testBufWriter{}
	eb := &testBufWriter{}
	outQ, errQ := newQueue(false, 100, ob, eb)

	outChan := make(chan string, 100) // Allowe plenty of buffer space so parent goroutine
	errChan := make(chan string, 100) // won't stall if tqbClient does

	outClient := &tqbClient{line: outChan, wtr: outQ}
	errClient := &tqbClient{line: errChan, wtr: errQ}
	defer close(outChan)
	defer close(errChan)

	go outClient.run()
	go errClient.run()

	fortyBytes := "aaaaaaaaaabbbbbbbbbbccccccccccdddddddddd"

	outChan <- fortyBytes // Should queue and return
	outChan <- fortyBytes // Should queue and return
	outChan <- fortyBytes // Should queue and black
	time.Sleep(time.Millisecond * 100)
	errChan <- fortyBytes // Should queue and black
	time.Sleep(time.Millisecond * 100)
	outChan <- fortyBytes // Channel should not be drained
	errChan <- fortyBytes // Channel should not be drained
	time.Sleep(time.Millisecond * 100)

	cl := len(outChan)
	if cl != 1 {
		t.Error("tqbClient should be blocked and unable to drain outChan", cl)
	}
	cl = len(errChan)
	if cl != 1 {
		t.Error("tqbClient should be blocked and unable to drain errChan", cl)
	}

	// Unblock queue

	outQ.foreground()
	time.Sleep(time.Millisecond * 100)
	cl = len(outChan)
	if cl != 0 {
		t.Error("tqbClient should be unblocked and unable to drain outChan", cl)
	}
	cl = len(errChan)
	if cl != 0 {
		t.Error("tqbClient should be unblocked and unable to drain errChan", cl)
	}
	outQ.close()
	errQ.close()
}

// First error should be returned and all subsequent errors ignored. Once a write fails,
// no further writes to the io.Writer should occur.
func TestQueueTransferOut(t *testing.T) {
	ob := &testTruncateWriter{}
	eb := &testTruncateWriter{}
	outQ, errQ := newQueue(false, 0, ob, eb)
	outQ.Write([]byte{'a', 'b', 'c', '\n'})
	outQ.Write([]byte{'x', 'y', 'z', '\n'})
	errQ.Write([]byte{'A', 'B', 'C', '\n'})
	errQ.Write([]byte{'X', 'Y', 'Z', '\n'})
	ob.append("stdout Write Fail", 1, errors.New("Stdout write failed"))
	eb.append("stderr should not be visible", 2, errors.New("Stderr write Failed"))
	e := outQ.cq.buf.transfer(ob, eb)
	if e == nil {
		t.Error("Expected error return from transfer")
	}
	res := ob.String()
	if res != "a" { // Should be one byte
		t.Error("Expected stdout to be 'a', not", res)
	}
	res = eb.String()
	if res != "AB" { // Should be two bytes
		t.Error("Expected stderr to be 'AB', not", res)
	}
}

// All of stdout should be written, but stderr should be truncated and return an error.
func TestQueueTransferErr(t *testing.T) {
	ob := &testTruncateWriter{}
	eb := &testTruncateWriter{}
	outQ, errQ := newQueue(false, 0, ob, eb)
	outQ.Write([]byte{'a'})
	outQ.Write([]byte{'b'})
	outQ.Write([]byte{'c'})
	outQ.Write([]byte{'d'})
	errQ.Write([]byte{'A', 'B', 'C', '\n'})
	errQ.Write([]byte{'X', 'Y', 'Z', '\n'})
	eb.append("stderr Write Fail", 3, errors.New("Stderr write failed"))
	e := outQ.cq.buf.transfer(ob, eb)
	if e == nil {
		t.Error("Expected error return from transfer")
	}
	res := ob.String()
	if res != "abcd" { // Should be one byte
		t.Error("Expected stdout to be 'abcd', not", res)
	}
	res = eb.String()
	if res != "ABC" { // Should be three bytes
		t.Error("Expected stderr to be 'ABC', not", res)
	}
}
