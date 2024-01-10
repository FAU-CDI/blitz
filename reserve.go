package blitz

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
)

// reserve reserves a slot in the highest-priority queue with the lowest delay.
// returns the index used, and the the reservation.
//
// if no queue with the given index exists, or all reservations fail, returns nil, -1.
func (blitz *Blitz) reserve(queue int) (*rate.Reservation, int) {
	// no such queue exists => bail out
	if queue < 0 || queue >= len(blitz.limiters) {
		return nil, -1
	}

	// make reservations for all the elements
	reservations := make([]*rate.Reservation, queue+1)

	// the index and value of the reservation with the lowest delay
	lowestDelayIndex := -1
	lowestDelayValue := rate.InfDuration

	// find the reservation with the lowest (or zero) delay
	nextInit := queue
	for nextInit >= 0 && lowestDelayValue > 0 {
		reservations[nextInit] = blitz.limiters[nextInit].Reserve()

		// if the delay is lower
		delay := reservations[nextInit].Delay()
		if delay < lowestDelayValue {
			lowestDelayIndex = nextInit
			lowestDelayValue = delay
		}

		// decrease the index
		nextInit--
	}

	// cancel all the non-picked reservations
	nextInit++
	for nextInit < len(reservations) {
		if nextInit != lowestDelayIndex {
			reservations[nextInit].Cancel()
		}
		nextInit++
	}

	// use the lowest delay
	if lowestDelayIndex >= 0 {
		return reservations[lowestDelayIndex], lowestDelayIndex
	} else {
		return nil, -1
	}
}

type reservation struct {
	Success             bool
	Queue               int
	DelayInMilliseconds int64

	XBlitzReservation string `json:"X-Blitz-Reservation"`

	TokenValidFromUnixMilliseconds  int64
	TokenValidUntilUnixMilliseconds int64
}

// signReservation creates and signs a reservation object for the given queue.
func (wrap *Blitz) signReservation(queue int) (rs reservation) {
	reserve, index := wrap.reserve(queue)
	if index == -1 {
		rs.Success = false
		return
	}
	rs.Queue = index
	rs.Success = true

	now := time.Now().UTC()

	delay := reserve.DelayFrom(now)
	from := now.Add(delay)
	to := from.Add(wrap.every)

	rs.DelayInMilliseconds = delay.Milliseconds()
	rs.TokenValidFromUnixMilliseconds = from.UnixMilli()
	rs.TokenValidUntilUnixMilliseconds = to.UnixMilli()

	// encode the reservation token
	rs.XBlitzReservation = wrap.signer.Encode(from, to)

	return
}

type errReservationExpired struct {
	ValidUntil, CurrentTime time.Time
}

func (err errReservationExpired) Error() string {
	return fmt.Sprintf("reservation expired: valid through %d, but it is now %d", err.ValidUntil.UnixMilli(), err.CurrentTime.UnixMilli())
}

// useReservation uses the given reservation.
//
// If a reservation is invalid, returns false.
// If a request is not yet valid, waits until it is, and then returns true.
func (wrap *Blitz) useReservation(ctx context.Context, token string) error {

	// decode the message
	validFrom, validUntil, err := wrap.signer.Decode(token)
	if err != nil {
		return err
	}

	// check validity
	now := time.Now().UTC()
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
		return errReservationExpired{ValidUntil: validUntil, CurrentTime: now}
	}
}
