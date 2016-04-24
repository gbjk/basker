package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

const (
	target = "http://127.0.0.1:8100/"
)

var workers int

func main() {

	flag.IntVar(&workers, "workers", 10, "Number of concurrent workers")
	flag.Parse()

	fmt.Println("Workers:", workers)

	feedbackPipe := make(chan float64)

	for i := 0; i < workers; i++ {
		go hitTarget(feedbackPipe)
	}

	for {
		select {
		case timing := <-feedbackPipe:
			fmt.Println("Got back: ", timing)
		}
	}
}

func hitTarget(feedbackPipe chan float64) {
	c := &fasthttp.Client{}

	for {
		start := time.Now()
		status, body, err := c.Get(nil, target)
		took := time.Since(start)

		if err != nil {
			log.Fatal("Unexpected error in http get: ", err)
		}

		if status != 200 {
			log.Fatal("Unexpected response code: ", status)
		}

		if expected, err := strconv.ParseFloat(strings.TrimSpace(string(body)), 64); err != nil {
			log.Print("Error parsing timing:", err)
		} else {
			expected := time.Duration(expected * float64(time.Millisecond))
			overhead := took - expected
			feedbackPipe <- overhead.Seconds()
		}
	}
}
