package main

import (
	"crypto/rand"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/fau-cdi/blitz"
)

//go:generate gogenlicense -m

func main() {
	u, err := url.Parse(redirectTarget)
	if err != nil {
		panic(err)
	}

	// create a proxy and a wrapper around it
	proxy := httputil.NewSingleHostReverseProxy(u)
	handler, err := blitz.New(rand.Reader, proxy, time.Second, qrates)
	if err != nil {
		panic(err)
	}

	// and start an http server
	log.Printf("Proxying %s to %s at rates of %v / second \n", bindAddress, redirectTarget, qrates)
	http.ListenAndServe(bindAddress, handler)
}
