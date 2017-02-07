package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/miekg/dns"
)

var (
	// Domain handled by this server.
	Domain = "home.idontfixcomputers.com."

	// DefaultTTL of records returned by this server.
	DefaultTTL = 7200 // Two hours
)

// NameServer is a simple DNS server designed to be quickly configured and
// run inside of Docker. It allows configuring forwarders per query type.
type NameServer struct {
	domain     string
	records    map[string]map[dns.Qtype]string // name => qtype => value
	forwarders map[string]string               // qtype => forwarder
}

func (s *NameServer) Handle(w dns.ResponseWriter, r *dns.Msg) {
	log.Printf("Received request with %v questions", len(r.Question))
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = true // TODO(christian): Change this.
	answ := make([]dns.RR, 0)
	// For each QUESTION:
	for _, q := range r.Question {
		// Check name for existence in records
		if qvm, ok := s.records[q.Name]; ok {
			if v, stillOK := qvm[q.Qtype]; stillOK {
				log.Printf("I should know how to handle %v for %q", q.Qtype, q.Name)
			}
		}
		// Don't have it? Forward that sucker.
	}
}

func handle(w dns.ResponseWriter, r *dns.Msg) {
	log.Printf("Received request with %v questions", len(r.Question))
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = true
	answ := make([]dns.RR, 0)

	for _, q := range r.Question {
		var rec dns.RR
		switch q.Qtype {
		case dns.TypeA:
			rec = &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(DefaultTTL),
				},
				A: net.ParseIP("10.42.6.254"),
			}
		case dns.TypeAAAA:
			rec = &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(DefaultTTL),
				},
				AAAA: net.ParseIP("2602:100:615f:f53e:a2a:6fe::"),
			}
		}
		if rec != nil {
			answ = append(answ, rec)
			rec = nil
		}
	}

	m.Answer = answ
	log.Printf("Responding with:\n%+v", m)

	w.WriteMsg(m)
}

func main() {
	fmt.Println("Starting DNS server")
	dns.HandleFunc(Domain, handle)
	go func() {
		srv := &dns.Server{Addr: ":53", Net: "udp"}
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()
	go func() {
		srv := &dns.Server{Addr: ":53", Net: "tcp"}
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatalf("Failed to set tcp listener %s\n", err.Error())
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	fmt.Printf("Signal (%s) received, stopping\n", s)
}
