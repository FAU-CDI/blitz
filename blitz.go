package blitz

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

var errAtLeastOneQueue = errors.New("at least one queue rate must be provided")

// Blitz creates a new blitz server wrapping handler
func New(rand io.Reader, handler http.Handler, every time.Duration, bs []uint64) (*Blitz, error) {
	if len(bs) == 0 {
		return nil, errAtLeastOneQueue
	}

	blitz := &Blitz{
		every:   every,
		Handler: handler,
	}

	blitz.limiters = make([]*rate.Limiter, len(bs))
	blitz.stats = make([]*Stats, len(bs))
	for i, b := range bs {
		blitz.limiters[i] = rate.NewLimiter(rate.Every(time.Second), int(b))
		blitz.stats[i] = NewStats(10 * every)
	}

	signer, err := newSigner(rand)
	if err != nil {
		return nil, err
	}
	blitz.signer = signer

	return blitz, nil
}

type Blitz struct {
	every time.Duration // how often the rate refills

	// limiters and statistics for each queue
	limiters []*rate.Limiter
	stats    []*Stats

	signer signer

	Logger  *log.Logger
	Handler http.Handler
}

type Status struct {
	Slots  []int64
	Delays []int64
}

func (blitz *Blitz) Status() (st Status) {
	// compute available slots for each queue
	st.Slots = make([]int64, len(blitz.limiters))
	for i, l := range blitz.limiters {
		st.Slots[i] = int64(math.Floor(l.Tokens()))
	}

	// compute the average delay for each queue
	st.Delays = make([]int64, len(blitz.limiters))
	for i, s := range blitz.stats {
		a, _ := s.Average().Int64()
		st.Delays[i] = time.Duration(a).Milliseconds()
	}

	return
}

const (
	HeaderReservation = "X-Blitz-Reservation"
	HeaderQueue       = "X-Blitz-Queue"
)

func (wrap *Blitz) logF(fmt string, args ...any) {
	if wrap.Logger == nil {
		log.Printf(fmt, args...)
		return
	}
	wrap.Logger.Printf(fmt, args...)
}

// getQueueHeader returns the header indicating the current queue.
// If no such header is present, it is of invalid format, or in-bounds checking fails, returns 0.
func (blitz *Blitz) getQueueHeader(r *http.Request) int {
	header := r.Header.Get(HeaderQueue)
	value, err := strconv.ParseInt(header, 10, 64)
	if err != nil {
		return 0
	}
	if value < 0 || value >= int64(len(blitz.limiters)) {
		return 0
	}
	return int(value)
}

func (blitz *Blitz) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/blitz/" {
		switch r.Method {
		case http.MethodGet:
			blitz.serveStatus(w, r)
		case http.MethodPost:
			blitz.serveReservation(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// passing the 'X-Blitz-Reservation' indicates that we reserved in the past.
	// we trust that the client has delayed accordingly.
	if reservation := r.Header.Get(HeaderReservation); reservation != "" {
		blitz.serveUseReservation(reservation, w, r)
		return
	}

	blitz.serveRegular(w, r)

}

func (blitz *Blitz) serveStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blitz.Status())
}

func (blitz *Blitz) serveReservation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reservation := blitz.signReservation(blitz.getQueueHeader(r))

	// if the reservation was a success,
	if reservation.Success {
		delay := time.Duration(reservation.DelayInMilliseconds * int64(time.Millisecond))
		blitz.logF("client %q on queue %d delay %s", r.RemoteAddr, reservation.Queue, delay)
		blitz.stats[reservation.Queue].AddInt64(delay.Nanoseconds())
	}

	json.NewEncoder(w).Encode(reservation)
}

func (blitz *Blitz) serveUseReservation(reservation string, w http.ResponseWriter, r *http.Request) {
	// validate the request
	if err := blitz.useReservation(r.Context(), reservation); err != nil {
		blitz.logF("client %q bad reservation: %v", r.RemoteAddr, err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad Request: %v\n", err)

		return
	}

	// delete the special headers
	r.Header.Del(HeaderReservation)
	r.Header.Del(HeaderQueue)

	// and forward the request
	blitz.Handler.ServeHTTP(w, r)
}

func (blitz *Blitz) serveRegular(w http.ResponseWriter, r *http.Request) {
	reservation, index := blitz.reserve(blitz.getQueueHeader(r))
	if index == -1 {
		blitz.logF("client %q delay ∞", r.RemoteAddr)
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "∞ delay")

		return
	}

	// check that we have a finite delay to wait
	delay := reservation.Delay()
	if delay == rate.InfDuration {
		blitz.logF("client %q delay ∞", r.RemoteAddr)

		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "∞ delay")
		return
	}

	// log the delay
	blitz.logF("client %q on queue %d delay %s", r.RemoteAddr, index, delay)
	blitz.stats[index].AddInt64(delay.Nanoseconds())

	// wait for the delay or the request to expire
	// whichever happens first
	select {
	case <-r.Context().Done():
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "Request cancelled by client")
	case <-time.After(delay):
		// delete the special headers
		r.Header.Del(HeaderReservation)
		r.Header.Del(HeaderQueue)

		// and forward
		blitz.Handler.ServeHTTP(w, r)
	}
}
