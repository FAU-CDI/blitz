package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/fau-cdi/blitz"
)

//go:generate gogenlicense -m

func main() {

	rps := flag.Int("rate-per-second", 10, "number of allowed requests per second")
	redirectTarget := flag.String("target", "", "redirect target")
	bindAddress := flag.String("bind", "127.0.0.1:8080", "address to bind to")
	legalFlag := flag.Bool("legal", false, "print legal notices and exit")
	flag.Parse()

	if *legalFlag {
		fmt.Println("This executable contains code from several different go packages. ")
		fmt.Println("Some of these packages require licensing information to be made available to the end user. ")
		fmt.Println(Notices) // this references the generated constant!
		os.Exit(0)
	}

	// parse the redirect target
	if *redirectTarget == "" {
		panic("no redirect target")
	}

	u, err := url.Parse(*redirectTarget)
	if err != nil {
		panic(err)
	}

	// create a proxy and a wrapper around it
	proxy := httputil.NewSingleHostReverseProxy(u)
	handler, err := blitz.New(rand.Reader, proxy, time.Second, *rps)
	if err != nil {
		panic(err)
	}

	// and start an http server
	log.Printf("Proxying %s to %s at a rate of %d / second \n", *bindAddress, *redirectTarget, *rps)
	http.ListenAndServe(*bindAddress, handler)
}
