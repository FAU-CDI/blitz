package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var qrates queues
var redirectTarget string
var bindAddress string = "127.0.0.1:8080"
var legalFlag bool

func init() {
	flag.Var(&qrates, "queue", "number of allowed requests per second")
	flag.StringVar(&redirectTarget, "target", redirectTarget, "target to proxy to")

	flag.StringVar(&bindAddress, "bind", bindAddress, "address to bind to")
	flag.BoolVar(&legalFlag, "legal", legalFlag, "print legal notices and exit")

	flag.Parse()

	if legalFlag {
		fmt.Println("This executable contains code from several different go packages. ")
		fmt.Println("Some of these packages require licensing information to be made available to the end user. ")
		fmt.Println(Notices)
		os.Exit(0)
	}

	// parse the redirect target
	if redirectTarget == "" {
		panic("no redirect target")
	}
}

// Created so that multiple integers (for rps) can be accepted
type queues []uint64

func (q *queues) String() string {
	if q == nil {
		return "<nil>"
	}

	flags := make([]string, len(*q))
	for i, q := range *q {
		flags[i] = strconv.FormatUint(q, 10)
	}
	return strings.Join(flags, ",")
}

func (q *queues) Set(value string) error {
	u, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return err
	}
	*q = append(*q, u)
	return nil
}
