package dnsserver

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"time"

	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
)

func (s *Server) parseQuery(m *dns.Msg) {
	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeA, dns.TypeAAAA:
			log.Printf("DNS Query for %s\n", q.Name)
			rawIp, found := s.cache.Get(q.Name)
			if found {
				ip := rawIp.(string)
				if ip != "" {
					rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
					if err == nil {
						m.Answer = append(m.Answer, rr)
					}
				}
			}
		}
	}
}

func (s *Server) handleDnsRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		s.parseQuery(m)
	}

	if err := w.WriteMsg(m); err != nil {
		slog.Warn("error handling dns response", "err", err)
	}
}

type Server struct {
	cache *cache.Cache
}

func (s *Server) ListenAndServe(ctx context.Context) {
	s.cache = cache.New(cache.NoExpiration, time.Hour*24)
	// attach request handler func
	dns.HandleFunc("service.", s.handleDnsRequest)

	port := 5353
	server := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp"}
	log.Printf("Starting at %d\n", port)

	go func() {
		<-ctx.Done()
		_ = server.Shutdown()
	}()

	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %s\n ", err.Error())
	}
}

func (s *Server) Add(ip string, name string) {
	s.cache.Set(name, ip, cache.NoExpiration)
}

func (s *Server) Del(name string) {
	s.cache.Delete(name)
}

func (s *Server) Clear() {
	s.cache = cache.New(cache.NoExpiration, time.Hour*24)
}

func (s *Server) Find(name string) string {
	ret, found := s.cache.Get(name)
	if !found {
		return ""
	}

	return ret.(string)
}
