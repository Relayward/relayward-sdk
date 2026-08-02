package contract

import "testing"

func TestSupports(t *testing.T) {
	if !Supports([]uint32{1, 2}, 2) {
		t.Fatal("expected version 2 to be supported")
	}
	if Supports([]uint32{1, 2}, 3) {
		t.Fatal("did not expect version 3 to be supported")
	}
}
