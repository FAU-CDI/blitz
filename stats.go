package blitz

import (
	"math/big"
	"slices"
	"sync"
	"time"
)

// Stats averages a set of values over the last period d.
// The zero value is not ready for use, see [NewStats].
type Stats struct {
	d time.Duration

	m         sync.Mutex // held when writing
	lastPurge time.Time

	// entries added, guaranteed to be weakly monotone
	entries []statElement
}

// NewStats creates a new stats object that holds statistics for the given duration.
func NewStats(d time.Duration) *Stats {
	return &Stats{d: d, lastPurge: time.Now()}
}

type statElement struct {
	time  time.Time
	value big.Float
}

// purge purges invalid elements.
func (s *Stats) purge() {
	// instance entries are valid until
	s.lastPurge = time.Now()

	// get the first valid index
	validIndex := slices.IndexFunc(s.entries, func(se statElement) bool {
		return s.lastPurge.Sub(se.time) <= s.d
	})

	switch {
	// nothing is valid
	case validIndex == -1:
		s.entries = s.entries[:0]

	// there are some invalid indexes
	case validIndex > 0:
		// move the entries to the left
		count := copy(s.entries, s.entries[validIndex:])
		s.entries = s.entries[:count]
	}

}

// Add adds a new value to be averaged for the current time.
func (s *Stats) Add(value *big.Float) {
	s.add(func(new *big.Float) { value.Set(value) })
}

// AddInt64 is like Add, but takes an int64
func (s *Stats) AddInt64(value int64) {
	s.add(func(new *big.Float) { new.SetInt64(value) })
}

func (s *Stats) add(f func(value *big.Float)) {
	s.m.Lock()
	defer s.m.Unlock()

	s.entries = append(s.entries, statElement{
		time: time.Now(),
	})
	f(&s.entries[len(s.entries)-1].value)

	if time.Since(s.lastPurge) > s.d {
		s.purge()
	}
}

// Average returns the average values added over the past d duration.
func (s *Stats) Average() *big.Float {
	s.m.Lock()
	defer s.m.Unlock()

	s.purge()

	// get the total number of entries
	var total big.Float
	if len(s.entries) == 0 {
		return &total
	}
	total.SetInt64(int64(len(s.entries)))

	// sum all the numbers
	var result big.Float
	for _, e := range s.entries {
		result.Add(&result, &e.value)
	}

	// divide by the total
	result.Quo(&result, &total)

	// and return
	return &result
}
