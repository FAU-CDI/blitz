# Blitz

Blitz - Bypassing Limits with Intelligent Technical Zen - is a proxy to deal with rate limiting APIs.

By default, this acts as a transparent REST proxy that throttles requests to a certain rates.
When a certain rate limit is exceeded, the proxy connection will be kept active until it can be forwarded. 
The rate limit can be configured via command line arguments.


## Building

Build this using standard [go](https://go.dev/) tools, version 1.21 or above.

For example, to build a static linux binary:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o blitz ./cmd/blitz
```

You can also run on the current operating system using:

```bash
go run ./cmd/blitz
```

## Running

To run the executable, either compile on-demand using `go run`, or run the built executable.

For example, to forward requests at a rate of at most 10 requests / second use:

```bash
./blitz -target https://example.com/ -queue 10
```

By default the executable will listen on port `8080` on `127.0.0.1`.
This can be changed using command line flags, run `./blitz -help` to see a complete list of command line flags.

## Status API

Clients can request the current status by making a `GET` request to `/blitz/`.
They get back a JSON object describing the current status:

```json
{
    // the current number of available slots for each queue
    "Slots":[1],

    // the average delay received by clients over the past 10 seconds, for each queue.
    // note that if there are only reservations this may be zero despite no forwards.
    "Delays": [0],
}
```

## Requesting a slot

Clients can also (non-transparently) "reserve" a forwarding slot by making a `POST` request to `/blitz/`. 
They receive back a json object containing a number of milliseconds to wait, and a signed key to submit in a `X-Blitz-Reservation` header at that time.

```json
{
    // was the request successful
    "Success":true,

    // the actual queue that was used for the reservation.
    // the is queue with the lowest delay, at most what the client requested.
    "Queue": 0,
    
    // delay from now until the reservation becomes valid, in milliseconds.
    "DelayInMilliseconds":0,
    
    // string to pass into the "X-Blitz-Reservation" header to use.
    "X-Blitz-Reservation":"VF8zc/FkSTBCDWj8Nn9fba3+Uc84leJ9Np0LwJaEGddaHZnw6Q3iV+7UOZrUTuHQW8UStDrbwYojZc4X56nbBMHsIj+MAQAAqfAiP4wBAAA",
    
    // time the token is valid from and until, unix timestamp in milliseconds.
    "TokenValidFromUnixMilliseconds":0,
    "TokenValidUntilUnixMilliseconds":0,
}
```

Afterwards, making any request to the proxy with the `X-Blitz-Reservation` header set to the provided string makes use of the reservation.
A call with an invalid reservation header results in an error.

## Multiple slots

Blitz supports running multiple prioritized queues.
To start it with multiple queues, simply pass the `-queue` command line flag multiple times.

Then send requests with the `X-Blitz-Queue` header to select a queue.
For example, passsing `X-Blitz-Queue` with a value of `0` will select the first queue.

Queues with higher indexes are considered higher priority. 
If the wait time on a higher queue is longer, the client will be automatically pushed to a lower queue.

Note that blitz reservations only need to pass the header when making the reservation, not when using it.

## LICENSE

See [LICENSE](LICENSE)