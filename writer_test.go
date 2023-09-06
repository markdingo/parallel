package parallel

import (
	"testing"
)

type testWriter struct {
	commonWriter
}

func (mtw *testWriter) Write(b []byte) (int, error) { return len(b), nil }
func (mtw *testWriter) close()                      {}

func TestCommonWriter(t *testing.T) {
	var cw1, cw2 testWriter
	cw1.out = &cw2
	if cw1.getNext() != &cw2 {
		t.Error("set/get disagree")
	}
}
