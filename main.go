package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	idb "github.com/influxdata/influxdb/client/v2"
	"github.com/valyala/fasthttp"
)

const (
	// TODO - move to flag
	influxdbUrl  = "http://192.168.99.101:8086"
	influxdbName = "proxy_overhead"
)

var workers int
var target string
var label string

func main() {

	flag.StringVar(&label, "label", "", "influxdb label")
	flag.StringVar(&target, "target", "", "http target uri incuding port")
	flag.IntVar(&workers, "workers", 10, "Number of concurrent workers")
	flag.Parse()

	fmt.Println("Workers:", workers)
	fmt.Println("Target:", target)
	fmt.Println("Label:", label)

	feedbackPipe := make(chan float64)

	if target == "" {
		log.Fatal("Please supply --target in the format http://127.1:8100/")
	}

	if label == "" {
		log.Fatal("Please supply --label")
	}

	for i := 0; i < workers; i++ {
		go hitTarget(feedbackPipe)
	}

	client, err := idb.NewHTTPClient(idb.HTTPConfig{
		Addr: influxdbUrl,
	})
	if err != nil {
		panic(err)
	}

	q := idb.NewQuery(fmt.Sprintf("CREATE DATABASE %s", influxdbName), "", "")
	// Ignoring errors, probably that the db already exists. TODO.
	client.Query(q)

	bp, err := idb.NewBatchPoints(idb.BatchPointsConfig{
		Precision: "ns",
		Database:  influxdbName,
	})

	if err != nil {
		panic(err)
	}

	for {
		select {
		case timing := <-feedbackPipe:
			fmt.Println("Got back: ", timing)
			tags := map[string]string{
				"backend": label,
			}
			fields := map[string]interface{}{
				"overhead": timing,
			}
			pt, err := idb.NewPoint("proxy_testing", tags, fields, time.Now())
			if err != nil {
				panic(err)
			}
			bp.AddPoint(pt)
			if err := client.Write(bp); err != nil {
				panic(err)
			}
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
