package metrics

import "testing"

func TestRingBufferPartial(t *testing.T) {
	r := NewRingBuffer(4)
	r.Push(1)
	r.Push(2)
	got := r.Values()
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("partial values = %v", got)
	}
	if r.Len() != 2 {
		t.Errorf("Len = %d, want 2", r.Len())
	}
}

func TestRingBufferWrap(t *testing.T) {
	r := NewRingBuffer(3)
	for _, v := range []float64{1, 2, 3, 4, 5} {
		r.Push(v)
	}
	got := r.Values()
	want := []float64{3, 4, 5}
	if len(got) != 3 {
		t.Fatalf("wrapped len = %d", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("wrapped values = %v, want %v", got, want)
			break
		}
	}
	if r.Len() != 3 {
		t.Errorf("Len = %d, want 3", r.Len())
	}
}

func TestRingBufferDefaultSize(t *testing.T) {
	r := NewRingBuffer(0)
	if len(r.data) != HistorySize {
		t.Errorf("default size = %d, want %d", len(r.data), HistorySize)
	}
}
