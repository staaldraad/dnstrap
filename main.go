package main

import (
        "flag"
        "fmt"
        "github.com/miekg/dns"
        "os"
        "strconv"
        "syscall"
        "os/signal"
        "net"
)

func handleReflect(w dns.ResponseWriter, r *dns.Msg) {
        m := new(dns.Msg)
        m.SetReply(r)
        m.Compress = false

        switch r.Opcode {
        case dns.OpcodeQuery:
                parseQuery(m,w.RemoteAddr())
        }

        w.WriteMsg(m)
}

func parseQuery(m *dns.Msg, remote net.Addr) {
        for _, q := range m.Question {
                switch q.Qtype {
                case dns.TypeA:
                        fmt.Printf("Query for [%s] from [%s]\n", q.Name,remote.String())
                        ip := "1.1.1.1" 
                        if ip != "" {
                                rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
                                if err == nil {
                                        m.Answer = append(m.Answer, rr)
                                }
                        }
                }
        }
}

func main() {
        rootDomain := flag.String("domain", "", "root domain to use (example: mydomain.pw.)")
        flag.Parse()

        if *rootDomain == "" {
                fmt.Println("-domain required")
                os.Exit(1)
        }

        dns.HandleFunc(*rootDomain, handleReflect)

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


