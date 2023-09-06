package parallel

import (
	"testing"
)

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

// No partial line writes, etc
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
