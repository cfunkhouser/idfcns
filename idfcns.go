package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"io/ioutil"

	"encoding/json"

	"github.com/miekg/dns"
)

var (
	domain   = flag.String("domain", ".", "Domain to be handled by this server.")
	address  = flag.String("address", ":53", "Host-port on which to listen (both UDP and TCP.)")
	confFile = flag.String("config", "", "Path to the configuration file.")
)

// ForwarderConfig contains all options for creating a QTypeForwarder.
type ForwarderConfig struct {
	// ServerOverrides maps query types to an alternate set of Servers to search.
	ServerOverrides map[string][]string `json:"qtype_overrides"`

	// The default DNS servers for forwarded requests.
	Servers []string `json:"servers"`
}

// ForwarderConfigFromJSON gets a ForwarderConfig from a JSON file.
func ForwarderConfigFromJSON(path string) (*ForwarderConfig, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &ForwarderConfig{}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	return c, nil
}

// QTypeForwarder is a DNS forwarder which forwards DNS queries to a particular
// upstream DNS server based on the query type.
type QTypeForwarder struct {
	forwarders map[uint16][]string // qtype => server
	catchall   []string            // default forwarder
	c          *dns.Client
}

// NewQTypeForwarder creates a new QTypeForwarder.
func NewQTypeForwarder(config *ForwarderConfig) *QTypeForwarder {
	forwarders := make(map[uint16][]string)
	if config.ServerOverrides == nil || len(config.ServerOverrides) == 0 {
		log.Printf("empty overrides, all requests going to %v", config.Servers)
	} else {
		log.Printf("default: %v", config.Servers)
		for q, s := range config.ServerOverrides {
			log.Printf("override %v => %v", q, s)
			forwarders[dns.StringToType[q]] = s
		}
	}
	return &QTypeForwarder{
		forwarders: forwarders,
		catchall:   config.Servers,
		c:          &dns.Client{},
	}
}

// Handle a DNS query. Meant to be passed to dns.HandleFunc.
func (f *QTypeForwarder) Handle(w dns.ResponseWriter, req *dns.Msg) {
	// Handle questions. Because we are sorting on QType, we need to break out
	// messages with multiple questions into multiple messages with one question
	// each. A future optimization might be to group questions with the same
	// QType into the same message if it turns out this is a bottleneck.
	answ := make([]dns.RR, 0)
	var rcode int
	for _, q := range req.Question {
		m := req.Copy()
		m.Question = []dns.Question{q}
		server := f.serverForQuestion(&q)
		r, _, err := f.c.Exchange(m, net.JoinHostPort(server, "53"))
		if err != nil {
			log.Printf("query errored: %v\nquery: %+v", err, m)
			rcode = dns.RcodeServerFailure
			break
		}
		if r.Rcode != dns.RcodeSuccess {
			log.Printf("query did not succeed: %v\nquery: %+v", dns.RcodeToString[r.Rcode], m)
			rcode = r.Rcode // Copy the first error and bail.
			break
		}
		answ = append(answ, r.Answer...)
	}

	resp := &dns.Msg{}
	resp.SetReply(req)
	if rcode != dns.RcodeSuccess {
		resp.Rcode = rcode
	} else {
		resp.Answer = answ
		resp.Authoritative = false
	}
	log.Printf("Responding with:\n%+v", resp)
	w.WriteMsg(resp)
}

func (f *QTypeForwarder) serverForQuestion(q *dns.Question) string {
	// TODO(christian): Don't ignore all but the first server.
	if s, ok := f.forwarders[q.Qtype]; ok {
		return s[0]
	}
	return f.catchall[0]
}

func main() {
	flag.Parse()

	config, err := ForwarderConfigFromJSON(*confFile)
	if err != nil {
		log.Fatalf("Couldn't load config file %q: %v", *confFile, err)
	}
	f := NewQTypeForwarder(config)
	dns.HandleFunc(*domain, f.Handle)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Starting DNS forwarder")
	for _, p := range []string{"udp", "tcp"} {
		go func(proto string) {
			srv := &dns.Server{Addr: *address, Net: proto}
			err := srv.ListenAndServe()
			if err != nil {
				log.Fatalf("Failed to create %v listener: %v", proto, err)
			}
		}(p)
	}

	s := <-sig
	fmt.Printf("Signal (%s) received, stopping\n", s)
}
