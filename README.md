# Blitz

Blitz - Bypassing Limits with Intelligent Technical Zen - is a proxy to deal with rate limiting APIs.

By default, this acts as a transparent REST proxy that throttles requests to a certain rates.
When a certain rate limit is exceeded, the proxy connection will be kept active until it can be forwarded. 
The rate limit can be configured via command line arguments.

Clients can request the current status by making a `GET`` request to `/blitz/`.
They get back a JSON object describing the current status:

```json
{
    // the current number of available slots
    "Available":1,

    // the average delay received by clients over the past 10 seconds.
    // note that if there are no clients, this may be zero despite no free clients.
    "AverageDelayInMilliseconds": 0,
}
```

Clients can also (non-transparently) "reserve" a forwarding slot by making a `POST` request to `/blitz/`. 
They receive back a json object containing a number of milliseconds to wait, and a signed key to submit in a `X-Blitz-Reservation` header at that time.

```json
{
    // was the request successfull
    "Success":true,
    
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

## Usage

Build this using standard [go](https://go.dev/) tools, version 1.21 or above.

For example, to build a static linux binary:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o blitz ./cmd/blitz
```

You can also run on the current operating system using:

```bash
go run ./cmd/blitz
```

Run `go run ./cmd/blitz -help` to see command line flags.


## LICENSE

See [LICENSE](LICENSE)