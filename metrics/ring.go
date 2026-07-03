package metrics

// RingBuffer keeps the last N float64 samples for sparkline rendering.
type RingBuffer struct {
	data []float64
	next int
	full bool
}

// HistorySize is the number of samples kept per metric
// (~2 minutes at the default 2s poll interval).
const HistorySize = 60

func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = HistorySize
	}
	return &RingBuffer{data: make([]float64, size)}
}

func (r *RingBuffer) Push(v float64) {
	r.data[r.next] = v
	r.next++
	if r.next == len(r.data) {
		r.next = 0
		r.full = true
	}
}

// Values returns the stored samples ordered oldest to newest.
func (r *RingBuffer) Values() []float64 {
	if !r.full {
		out := make([]float64, r.next)
		copy(out, r.data[:r.next])
		return out
	}
	out := make([]float64, 0, len(r.data))
	out = append(out, r.data[r.next:]...)
	out = append(out, r.data[:r.next]...)
	return out
}

// Len returns the number of samples currently stored.
func (r *RingBuffer) Len() int {
	if r.full {
		return len(r.data)
	}
	return r.next
}
