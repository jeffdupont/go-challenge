package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const maxConnections = 10
const iso8601Format = "2006-01-02T15:04:05Z"

// Metric represents the parsed input data and keeps track of the count and
// mean value of all metrics in the current collection and the last
// timestamp inserted
type metric struct {
	name  string
	value float64
	mean  float64
	time  time.Time
	count int
}

// Store saves all metric data and relies on the RW Mutex to ensure
// that all metric names are distinct. I used RW to allow concurrent reads
// when check that the key exists before locking to save the metric
type store struct {
	data map[string]metric
}

// Initializes the store db for the metric data
func newStore() *store {
	return &store{make(map[string]metric)}
}

// Update checks to see if the metric key exists
// and then updates the existing value before it is saved
// back to the data store
func (s *store) update(m metric) error {
	// check if the metric exists
	if _, ok := s.data[m.name]; ok {
		cm := s.data[m.name]
		m.value = cm.value + m.value
		m.count = cm.count + 1
		m.mean = m.value / float64(m.count)
	}
	s.data[m.name] = m
	return nil
}

var (
	currentConnections uint64
	rawCount           uint64
)

// Make sure the name contains only valid characters
func validateName(str string) bool {
	if len(str) > 64 {
		fmt.Println("invalid input: too big")
		return false
	}
	for i, r := range str {
		// Make sure that the '-' is not the first char in the string
		if i == 0 && r == '-' {
			return false
		}
		// Using the ASCII values, we can determine if each rune is valid. We first
		// check to see if it is outside the given ranges for 0-9, A-Z, and a-z
		// and then finally make sure that it's not '-'
		if !(r >= '0' && r <= '9') && !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && r != '-' {
			return false
		}
	}
	return true
}

// Parse the input line
func parseMetric(line string) (*metric, error) {
	data := strings.Split(line, "\t")
	if len(data) != 3 {
		return nil, fmt.Errorf("invalid input: missing values")
	}

	// validate name
	name := data[0]
	if ok := validateName(name); !ok {
		return nil, fmt.Errorf("invalid input: name ")
	}

	// validate value
	v, err := strconv.ParseFloat(data[1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid input: value not float")
	}

	// validate time
	t, err := time.Parse(iso8601Format, data[2])
	if err != nil {
		return nil, fmt.Errorf("invalid input: time not iso8601")
	}

	return &metric{name: name, value: v, mean: v, time: t, count: 1}, nil
}

type empty struct{}
type semaphore chan empty

// acquire n resources
func (s semaphore) P(n int) {
	e := empty{}
	for i := 0; i < n; i++ {
		s <- e
	}
}

// release n resources
func (s semaphore) V(n int) {
	for i := 0; i < n; i++ {
		<-s
	}
}

func (s semaphore) Signal() {
	s.V(1)
}

func (s semaphore) Wait(n int) {
	s.P(n)
}

func main() {
	// initialize the main store db
	store := newStore()

	ingress := make(chan metric)
	// process feed and tickers
	go func() {
		tickerRaw := time.NewTicker(time.Second * 10)
		tickerCollection := time.NewTicker(time.Second * 30)
		for {
			select {
			case m := <-ingress:
				_ = store.update(m)
			case <-tickerRaw.C:
				fmt.Fprintf(os.Stderr, "(10 sec): Record count %d\n", atomic.LoadUint64(&rawCount))
				atomic.StoreUint64(&rawCount, 0) // reset the count
			case <-tickerCollection.C:
				// could use a text template here to display columns
				// but this is simple and efficient
				for _, m := range store.data {
					fmt.Fprintln(os.Stdout, m.name, "\t", m.mean)
				}
				store.data = make(map[string]metric) // empty the collection
			}
		}
	}()

	// establish the tcp listener
	l, err := net.Listen("tcp", ":4268")
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	sem := make(semaphore, 10)
	for {
		sem.Wait(1)
		conn, err := l.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Connection: %v\n", err)
			continue
		}
		go connHandler(conn, sem, ingress)
	}
}

// Handles all the data incoming for the given connection
func connHandler(conn net.Conn, s semaphore, ingress chan metric) {
	defer s.Signal()
	reader := bufio.NewReader(conn)

	for {
		// read the input
		b, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(os.Stderr, "client terminated: EOF")
				conn.Close()
				return
			}
		}

		// trim off unnecessary chars
		line := string(bytes.Trim(b, "\r\n"))
		if line == "" {
			fmt.Fprintln(os.Stderr, "client terminated: Empty input")
			conn.Close()
			return
		}

		// parse the metric
		metric, err := parseMetric(line)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			conn.Close()
			return
		}

		// if the record timestamp is outside the last minute then ignore it
		if metric.time.Before(time.Now().Add(-60*time.Second).UTC()) ||
			metric.time.After(time.Now()) {
			continue
		}

		// save the metric to the store
		ingress <- *metric

		// increment our raw 10 min counter
		atomic.AddUint64(&rawCount, 1)
	}
}
