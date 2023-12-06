package blitz

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"time"

	"golang.org/x/crypto/nacl/sign"
	"golang.org/x/time/rate"
)

// Blitz creates a new blitz server wrapping handler
func New(rand io.Reader, handler http.Handler, every time.Duration, b int) (*Blitz, error) {
	blitz := &Blitz{
		every: every,

		limiter: rate.NewLimiter(rate.Every(time.Second), b),
		stats:   NewStats(10 * every),
		Handler: handler,
	}

	puk, pik, err := sign.GenerateKey(rand)
	if err != nil {
		return nil, err
	}

	blitz.pubKey = puk
	blitz.privKey = pik

	return blitz, nil
}

type Blitz struct {
	every time.Duration // how often the rate refills

	limiter *rate.Limiter

	pubKey  *[32]byte
	privKey *[64]byte

	stats   *Stats
	logger  *log.Logger
	Handler http.Handler
}

type Status struct {
	// Number of tokens available right now.
	// A negative value indicates having to wait.
	AvailableSlots int64

	// AverageDelay is the average delay over the past 10 seconds
	AverageDelayInMilliseconds int64
}

func (wrap *Blitz) Status() (st Status) {
	st.AvailableSlots = int64(math.Floor(wrap.limiter.Tokens()))
	i, _ := wrap.stats.Average().Int64()
	st.AverageDelayInMilliseconds = time.Duration(i).Milliseconds()
	return
}

type rs struct {
	Success             bool
	DelayInMilliseconds int64

	XBlitzReservation string `json:"X-Blitz-Reservation"`

	TokenValidFromUnixMilliseconds  int64
	TokenValidUntilUnixMilliseconds int64
}

func (wrap *Blitz) makeReservation() (rs rs) {

	reserve := wrap.limiter.Reserve()
	if !reserve.OK() {
		rs.Success = false
		return
	}

	now := time.Now()

	delay := reserve.DelayFrom(now)
	from := now.Add(delay)
	to := from.Add(wrap.every)

	rs.DelayInMilliseconds = delay.Milliseconds()
	rs.TokenValidFromUnixMilliseconds = from.UnixMilli()
	rs.TokenValidUntilUnixMilliseconds = to.UnixMilli()

	// create a message [from, until]
	message := make([]byte, messageLength)
	binary.LittleEndian.PutUint64(message[:8], uint64(rs.TokenValidFromUnixMilliseconds))
	binary.LittleEndian.PutUint64(message[8:], uint64(rs.TokenValidUntilUnixMilliseconds))

	// sign the message with the private key
	signature := make([]byte, 0, signatureLength)
	signature = sign.Sign(signature, message, wrap.privKey)

	// encode it as base64
	rs.XBlitzReservation = base64.StdEncoding.EncodeToString(signature)
	return
}

var (
	errInvalidReservationFormat    = errors.New("invalid reservation format")
	errInvalidReservationSignature = errors.New("invalid reservation signature")
	errReservationExpired          = errors.New("reservation expired")
)

var (
	messageLength   = 2 * (64 / 8)                                   // length of the reservation, 2 bytes of 64 ints
	signatureLength = messageLength + sign.Overhead                  // length of message + signature
	encodedLength   = base64.StdEncoding.EncodedLen(signatureLength) // length of base64
)

// validReservation ensures that a reservation is valid
//
// If a reservation is invalid, returns false.
// If a request is not yet valid, waits until it is, and then returns true
func (wrap *Blitz) validReservation(ctx context.Context, token string) error {
	if len(token) != encodedLength {
		return errInvalidReservationFormat
	}

	// do the decode!
	signed, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return errInvalidReservationFormat
	}

	// verify the message
	message := make([]byte, 16)
	if _, valid := sign.Open(message, signed, wrap.pubKey); !valid {
		return errInvalidReservationSignature
	}

	// get valid (from, until) times
	validFrom := time.UnixMilli(int64(binary.LittleEndian.Uint64(message[:8])))
	validUntil := time.UnixMilli(int64(binary.LittleEndian.Uint64(message[8:])))

	now := time.Now()
	switch {
	// valid now!
	case now.After(validFrom) && now.Before(validUntil):
		return nil

		// not yet valid => wait until it is
	case now.Before(validFrom):
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(validFrom)):
			return nil
		}

	// signature expired
	default:
		return errReservationExpired
	}
}

func (wrap *Blitz) serveBlitz(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(wrap.Status())

	case http.MethodPost:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(wrap.makeReservation())

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (wrap *Blitz) logF(fmt string, args ...any) {
	if wrap.logger == nil {
		log.Printf(fmt, args...)
		return
	}
	wrap.logger.Printf(fmt, args...)
}

func (wrap *Blitz) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/blitz/" {
		wrap.serveBlitz(w, r)
		return
	}

	// passing the 'X-Blitz-Reservation' indicates that we reserved in the past.
	// we trust that the client has delayed accordingly.
	if reservation := r.Header.Get("X-Blitz-Reservation"); reservation != "" {
		// validate the request
		if err := wrap.validReservation(r.Context(), reservation); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Bad Request: %v\n", err)
			return
		}

		// forward the request
		r.Header.Del("X-Blitz-Request")
		wrap.Handler.ServeHTTP(w, r)
		return
	}

	// create a reservation
	reserve := wrap.limiter.Reserve()
	defer reserve.Cancel()

	// check that we have a finite delay to wait
	delay := reserve.Delay()
	if delay == rate.InfDuration {
		wrap.logF("client %q delay ∞", r.RemoteAddr)

		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "∞ delay")
		return
	}
	wrap.logF("client %q delay %s", r.RemoteAddr, delay)
	wrap.stats.AddInt64(delay.Nanoseconds())

	// wait
	select {
	case <-r.Context().Done():
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "Infinite Delay (wrong config?)")

		return
	case <-time.After(delay):
		/* see below */
	}

	// call the handler
	wrap.Handler.ServeHTTP(w, r)
}
