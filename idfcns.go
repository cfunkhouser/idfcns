package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
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
	mu         sync.RWMutex                 // Protects the following fields
	records    map[string]map[uint16]string // name => qtype => value
	forwarders map[string]string            // qtype => forwarder
}

func (s *NameServer) Handle(w dns.ResponseWriter, r *dns.Msg) {
	log.Printf("Received request with %v questions", len(r.Question))
	m := &dns.Msg{}
	m.SetReply(r)
	m.Authoritative = true // TODO(christian): Change this.
	answ := make([]dns.RR, 0)
	// For each QUESTION:
	for _, q := range r.Question {
		log.Printf("Looking for %q in %v", q.Name, q.Qtype)
		// Check name for existence in records
		var rec dns.RR
		if qvm, ok := s.records[q.Name]; ok {
			if v, stillOK := qvm[q.Qtype]; stillOK {
				log.Printf("I should know how to handle %v for %q", q.Qtype, q.Name)

				hdr := dns.RR_Header{
					Name:   q.Name,
					Rrtype: q.Qtype,
					Class:  dns.ClassINET,
					Ttl:    uint32(DefaultTTL),
				}

				// TODO(christian): this is gross, break it out.
				switch q.Qtype {
				case dns.TypeA:
					rec = &dns.A{
						Hdr: hdr,
						A:   net.ParseIP(v),
					}
				case dns.TypeAAAA:
					rec = &dns.AAAA{
						Hdr:  hdr,
						AAAA: net.ParseIP(v),
					}
				}
			}
		}
		// Don't have it? Forward that sucker.
		// TODO(christian): Forward that sucker.
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

	ns := &NameServer{
		records: map[string]map[uint16]string{
			"foo.home.idontfixcomputers.com.": map[uint16]string{
				dns.TypeA:    "10.42.6.254",
				dns.TypeAAAA: "2602:100:615f:f53e:a2a:6fe::",
			},
		},
	}

	dns.HandleFunc(Domain, ns.Handle)
	go func() {
		srv := &dns.Server{Addr: ":8053", Net: "udp"}
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()
	go func() {
		srv := &dns.Server{Addr: ":8053", Net: "tcp"}
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
