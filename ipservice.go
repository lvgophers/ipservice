package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"time"
)

var concurrency = flag.Int("c", 16, "Concurrency")
var listen = flag.String("addr", ":8080", "Listen address")

func init() {
	log.SetFlags(0)
}

type Service chan struct{}

func (s Service) done() {
	<-s
}

func (s Service) lookup(domain string) ([]net.IP, error) {
	done := make(chan struct{})
	var ips []net.IP
	var err error
	go func() {
		ips, err = net.LookupIP(domain)
		close(done)
	}()
	select {
	case <-done:
		return ips, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout")
	}
}

func (s Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	domain := path.Base(r.URL.Path)
	switch domain {
	case ".", "/":
		http.Error(w, "Invalid input", 500)
		return
	}
	select {
	case s <- struct{}{}:
	default:
		http.Error(w, "Throttled", 420)
		return
	}
	defer s.done()
	ips, err := s.lookup(domain)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(ips)
}

func main() {
	flag.Parse()
	s := make(Service, *concurrency)
	if err := http.ListenAndServe(*listen, s); err != nil {
		log.Fatal(err)
	}
}
