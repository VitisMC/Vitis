package lighting

import "testing"

func TestNibbleGetSet(t *testing.T) {
	var n NibbleArray

	n.Set(0, 5)
	if got := n.Get(0); got != 5 {
		t.Errorf("Get(0) = %d, want 5", got)
	}

	n.Set(1, 12)
	if got := n.Get(1); got != 12 {
		t.Errorf("Get(1) = %d, want 12", got)
	}

	if got := n.Get(0); got != 5 {
		t.Errorf("Get(0) after Set(1) = %d, want 5", got)
	}
}

func TestNibbleGetSetXYZ(t *testing.T) {
	var n NibbleArray

	n.SetXYZ(3, 7, 11, 15)
	if got := n.GetXYZ(3, 7, 11); got != 15 {
		t.Errorf("GetXYZ(3,7,11) = %d, want 15", got)
	}

	n.SetXYZ(0, 0, 0, 1)
	if got := n.GetXYZ(0, 0, 0); got != 1 {
		t.Errorf("GetXYZ(0,0,0) = %d, want 1", got)
	}
}

func TestNibbleIsEmpty(t *testing.T) {
	var n NibbleArray
	if !n.IsEmpty() {
		t.Error("new NibbleArray should be empty")
	}

	n.Set(100, 7)
	if n.IsEmpty() {
		t.Error("NibbleArray with data should not be empty")
	}
}

func TestNibbleFill(t *testing.T) {
	var n NibbleArray
	n.Fill(15)

	for i := 0; i < EntriesPerSection; i++ {
		if got := n.Get(i); got != 15 {
			t.Errorf("Get(%d) = %d after Fill(15), want 15", i, got)
			break
		}
	}
}

func TestNibbleClear(t *testing.T) {
	var n NibbleArray
	n.Fill(8)
	n.Clear()

	if !n.IsEmpty() {
		t.Error("NibbleArray should be empty after Clear")
	}
}

func TestNibbleOutOfRange(t *testing.T) {
	var n NibbleArray

	n.Set(-1, 5)
	n.Set(EntriesPerSection, 5)

	if got := n.Get(-1); got != 0 {
		t.Errorf("Get(-1) = %d, want 0", got)
	}
	if got := n.Get(EntriesPerSection); got != 0 {
		t.Errorf("Get(%d) = %d, want 0", EntriesPerSection, got)
	}
}

func TestNibbleClampTo4Bits(t *testing.T) {
	var n NibbleArray
	n.Set(0, 0xFF)
	if got := n.Get(0); got != 15 {
		t.Errorf("Get(0) = %d after Set(0, 0xFF), want 15", got)
	}
}

func TestNibbleBytes(t *testing.T) {
	var n NibbleArray
	b := n.Bytes()
	if len(b) != NibbleArraySize {
		t.Errorf("Bytes() length = %d, want %d", len(b), NibbleArraySize)
	}
}
