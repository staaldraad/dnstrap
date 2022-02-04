package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/miekg/dns"
)

type IKey interface{}
type IValue interface{}

type Records struct {
	entries map[IKey]IValue
	lock    sync.RWMutex
}

func (records *Records) Add(key IKey, value IValue) {
	records.lock.Lock()
	defer records.lock.Unlock()
	if records.entries == nil {
		records.entries = make(map[IKey]IValue)
	}
	records.entries[key] = value
}

func (records *Records) Remove(key IKey) bool {
	records.lock.Lock()
	defer records.lock.Unlock()
	_, ok := records.entries[key]
	if ok {
		delete(records.entries, key)
	}
	return ok
}

func (records *Records) Get(key IKey) IValue {
	records.lock.RLock()
	defer records.lock.RUnlock()
	return records.entries[key]
}

func (records *Records) Clear() {
	records.lock.Lock()
	defer records.lock.Unlock()
	records.entries = make(map[IKey]IValue)
}

var dnsRecords Records
var localDomain string

func handleReflect(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		response := false
		for _, q := range m.Question {
			switch q.Qtype {
			case dns.TypeA:
				fmt.Printf("Query for [%s] from [%s]\n", q.Name, w.RemoteAddr().String())
				// lookup
				if entry := dnsRecords.Get(q.Name); entry != nil {
					rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, entry))
					if err == nil {
						m.Answer = append(m.Answer, rr)
						response = true
					}
				} else { // we don't have this in memory, nxdomain
					// if request is for local subdomain
					if localDomain != "" && strings.HasSuffix(q.Name, localDomain) {
						m.SetRcode(r, dns.RcodeNameError)
						m.Ns = []dns.RR{soa(q.Name)}
						response = true
					} else {
						// recursive request
						break
					}
				}
			}
		}
		if response {
			w.WriteMsg(m)
			return
		}
		m1 := new(dns.Msg)
		m1.Id = m.Id
		m1.RecursionDesired = true
		m1.Question = make([]dns.Question, len(m.Question))
		for i, q := range m.Question {
			m1.Question[i] = q
		}

		//c := new(dns.Client)
		resp, err := dns.Exchange(m1, "1.1.1.1:53")
		if err != nil {
			fmt.Println(err)
			return
		}
		resp.Id = r.Id

		_ = w.WriteMsg(resp)
	}

}

func parseQuery(m *dns.Msg, r *dns.Msg, remote net.Addr) {

}

func soa(name string) dns.RR {
	s := fmt.Sprintf("%s 60 IN SOA ns1.%s postmaster.%s 1524370381 14400 3600 604800 60", name, name, name)
	soa, _ := dns.NewRR(s)
	return soa
}

func main() {
	selfDomain := flag.String("domain", "", "self domain, if we need to respond with NXDOMAIN (example: mydomain.pw.)")

	flag.Parse()

	if *selfDomain != "" {
		fmt.Printf("Unknown subdomains of [%s] will return NXDOMAIN", *selfDomain)
		localDomain = *selfDomain
	}

	dnsRecords = Records{}
	// insert initial
	dnsRecords.Add("blah.sub.conch.cloud.", "127.0.0.1")
	dnsRecords.Add("blah3.sub.conch.cloud.", "127.0.0.1")
	dns.HandleFunc(".", handleReflect)

	port := 53
	server := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp"}
	fmt.Printf("Starting at %d\n", port)
	go func() {
		err := server.ListenAndServe()
		defer server.Shutdown()
		if err != nil {
			fmt.Printf("Failed to start server: %s\n ", err.Error())
			os.Exit(1)
		}
	}()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	fmt.Printf("Signal (%s) received, stopping\n", s)
}
