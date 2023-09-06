package parallel

import (
	"testing"
	"time"
)

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
