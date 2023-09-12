package parallel

import (
	"errors"
	"testing"
)

// Test that an unconfigured tagger writer does not modify the data stream
func TestTaggerEmpty(t *testing.T) {
	var buf testBufWriter
	wtr := newTagger(&buf, []byte{})

	exp := "Line 1\nLine 2\nLine 3\n"
	b, e := wtr.Write([]byte(exp))
	if b != len(exp) {
		t.Error("Write len wrong. Got", b, "expected", len(exp))
	}
	if e != nil {
		t.Error("Unexpected error", e)
	}

	act := buf.String()
	if act != exp {
		t.Error("Unexpected modification. \nExp", exp, "\nactual", act)
	}
}

// Prepend with a whole line data
func TestTaggerSimple(t *testing.T) {
	var buf testBufWriter
	wtr := newTagger(&buf, []byte("host1: "))

	before := "Line 1\nLine 2\n"
	exp := "host1: Line 1\nhost1: Line 2\n"
	b, e := wtr.Write([]byte(before))
	if b != len(before) {
		t.Error("Write len wrong. Got", b, "expected", len(before))
	}
	if e != nil {
		t.Error("Unexpected error", e)
	}

	act := buf.String()
	if act != exp {
		t.Error("Unexpected modification. \nExp", exp, "\nactual", act)
	}
}

// Test tagger prepend logic with no trailing newline
func TestTaggerNoTrailingNL(t *testing.T) {
	var buf testBufWriter
	wtr := newTagger(&buf, []byte("host1: "))

	before := "Line 1\nXX"
	exp := "host1: Line 1\nhost1: XX"
	b, e := wtr.Write([]byte(before))
	if b != len(before) {
		t.Error("Write len wrong. Got", b, "expected", len(before))
	}
	if e != nil {
		t.Error("Unexpected error", e)
	}

	act := buf.String()
	if act != exp {
		t.Error("Unexpected modification. \nExp", exp, "\nactual", act)
	}
}

// Prepend with partial lines
func TestTaggerPartialWrites(t *testing.T) {
	var buf testBufWriter
	wtr := newTagger(&buf, []byte("host1: "))

	before := []byte("Line 1\nLine2 \nLine 3\nLine 4\n")
	exp := "host1: Line 1\nhost1: Line2 \nhost1: Line 3\nhost1: Line 4\n"
	for _, one := range before {
		b, e := wtr.Write([]byte{one})
		if e != nil {
			t.Fatal("Unexpected error", e)
		}
		if b != 1 {
			t.Error("Expected a 1 byte write, not", b)
		}
	}

	act := buf.String()
	if act != exp {
		t.Error("Unexpected modification. \nExp", exp, "\nactual", act)
	}
}

// Zero-length writes
func TestTaggerWriteZero(t *testing.T) {
	var buf testBufWriter
	wtr := newTagger(&buf, []byte("host1: "))

	before := []byte("")
	exp := ""
	b, e := wtr.Write(before)
	if e != nil {
		t.Fatal("Unexpected error", e)
	}
	if b != 0 {
		t.Error("Expected a zero byte write, not", b)
	}

	act := buf.String()
	if act != exp {
		t.Error("Unexpected modification. \nExp", exp, "\nactual", act)
	}
}

// Data is zero length
func TestTaggerIOErrorsW0(t *testing.T) {
	buf := &testTruncateWriter{}
	wtr := newTagger(buf, []byte(""))
	b, err := wtr.Write([]byte{})
	if b != 0 || err != nil {
		t.Error("Expected zero and nil, not", b, err)
	}
}

// Tag is zero length so return downstream write results
func TestTaggerIOErrorsW1(t *testing.T) {
	buf := &testTruncateWriter{}
	buf.append("Error on first write", -1, errors.New("W1 error"))
	wtr := newTagger(buf, []byte(""))
	b, err := wtr.Write([]byte{'a'})
	if b != 1 || err == nil {
		t.Error("Expected one and error, not", b)
	}
}

// Error with tag pending on first line
func TestTaggerIOErrorsW2(t *testing.T) {
	buf := &testTruncateWriter{}
	wtr := newTagger(buf, []byte("W2Tag"))
	buf.append("Error on first write", 0, errors.New("Tag Error"))
	b, err := wtr.Write([]byte{'a', '\n', 'b', '\n'})
	if b != 4 { // User bytes written is unaffected
		t.Error("Expected 4 bytes written, not", b)
	}
	if err == nil || err.Error() != "Tag Error" { // But error should be present
		t.Error("Expected Tag Error return, not", err)
	}
}

// Error on writing of first line
func TestTaggerIOErrorsW3(t *testing.T) {
	buf := &testTruncateWriter{}
	wtr := newTagger(buf, []byte("W3Tag"))
	buf.append("W1 is good", -1, nil)
	buf.append("W3 is error", -1, errors.New("W3 error"))

	data := []byte{'a', 'b', 'c', 'd', '\n', 'A', 'B', '\n'}
	b, err := wtr.Write(data)
	if b != len(data) {
		t.Error("Expected W3 to return", len(data), "not", b)
	}
	if err == nil || err.Error() != "W3 error" {
		t.Error("Expected 'W3 error', not", err)
	}
}

// Error on writing NL between split lines
func TestTaggerIOErrorsW4(t *testing.T) {
	buf := &testTruncateWriter{}
	wtr := newTagger(buf, []byte("W4Tag"))
	buf.append("W2Tag", -1, nil)
	buf.append("W3Line 1 'abcd'", -1, nil)
	buf.append("W4 NL fails", 0, errors.New("W4 NL failed"))

	data := []byte{'a', 'b', 'c', 'd', '\n', 'A', 'B', '\n'}
	b, err := wtr.Write(data)
	if b != len(data)-1 { // Minus the NL
		t.Error("Expected W4 to return", len(data)-1, "not", b)
	}
	if err == nil || err.Error() != "W4 NL failed" {
		t.Error("Expected 'W4 NL failed', not", err)
	}
}

// Error on writing tag on last line
func TestTaggerIOErrorsW5(t *testing.T) {
	buf := &testTruncateWriter{}
	wtr := newTagger(buf, []byte("W5Tag"))
	buf.append("W5 Tag fails", 2, errors.New("W5 Tag failed"))

	data := []byte{'a', 'b', 'c'} // Last line
	b, err := wtr.Write(data)
	if b != 3 { // Tag write failed, but data write succeeds
		t.Error("Expected W5 to return", 3, "not", b)
	}
	if err == nil || err.Error() != "W5 Tag failed" {
		t.Error("Expected 'W5 Tag failed', not", err)
	}
}

// Error on writing data on last line
func TestTaggerIOErrorsW6(t *testing.T) {
	buf := &testTruncateWriter{}
	wtr := newTagger(buf, []byte("W5Tag"))
	buf.append("W5 Tag ok", -1, nil)
	buf.append("W6 LL fails", 2, errors.New("W6 LL failed"))

	data := []byte{'a', 'b', 'c'} // Last line
	b, err := wtr.Write(data)
	if b != 2 { // Tag write ok, but data write failed
		t.Error("Expected W6 to return", 2, "not", b)
	}
	if err == nil || err.Error() != "W6 LL failed" {
		t.Error("Expected 'W6 LL failed', not", err)
	}
}
