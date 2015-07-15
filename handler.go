package main

import (
	"github.com/miekg/dns"
	"gitlab.sva.tuhh.de/svars/dnschan/dnschan"
	"net"
	"strings"
	"time"
)

const (
	notIPQuery = 0
	_IP4Query  = 4
	_IP6Query  = 6
)

type Question struct {
	qname  string
	qtype  string
	qclass string
}

func (q *Question) String() string {
	return q.qname + " " + q.qclass + " " + q.qtype
}

type GODNSHandler struct {
	resolver        *Resolver
	cache, negCache Cache
	hosts           Hosts
}

func NewHandler() *GODNSHandler {

	var (
		clientConfig    *dns.ClientConfig
		cacheConfig     CacheSettings
		resolver        *Resolver
		cache, negCache Cache
	)

	resolvConfig := settings.ResolvConfig
	clientConfig, err := dns.ClientConfigFromFile(resolvConfig.ResolvFile)
	if err != nil {
		logger.Printf(":%s is not a valid resolv.conf file\n", resolvConfig.ResolvFile)
		logger.Println(err)
		panic(err)
	}
	clientConfig.Timeout = resolvConfig.Timeout
	resolver = &Resolver{clientConfig}

	cacheConfig = settings.Cache
	switch cacheConfig.Backend {
	case "memory":
		cache = &MemoryCache{
			Backend:  make(map[string]Mesg, cacheConfig.Maxcount),
			Expire:   time.Duration(cacheConfig.Expire) * time.Second,
			Maxcount: cacheConfig.Maxcount,
		}
		negCache = &MemoryCache{
			Backend:  make(map[string]Mesg),
			Expire:   time.Duration(cacheConfig.Expire) * time.Second / 2,
			Maxcount: cacheConfig.Maxcount,
		}
	case "redis":
		// cache = &MemoryCache{
		// 	Backend:    make(map[string]*dns.Msg),
		//  Expire:   time.Duration(cacheConfig.Expire) * time.Second,
		// 	Serializer: new(JsonSerializer),
		// 	Maxcount:   cacheConfig.Maxcount,
		// }
		panic("Redis cache backend not implement yet")
	default:
		logger.Printf("Invalid cache backend %s", cacheConfig.Backend)
		panic("Invalid cache backend")
	}

	hosts := NewHosts(settings.Hosts, settings.Redis)

	return &GODNSHandler{resolver, cache, negCache, hosts}
}

func (h *GODNSHandler) do(Net string, w dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	Q := Question{UnFqdn(q.Name), dns.TypeToString[q.Qtype], dns.ClassToString[q.Qclass]}

	Debug("Question: %s", Q.String())

	IPQuery := h.isIPQuery(q)

	if IPQuery > 0 {
		if strings.HasSuffix(Q.qname, ".zz") {
			m := new(dns.Msg)
			m.SetReply(req)

			switch IPQuery {
			case _IP4Query:
				ip := net.ParseIP("127.0.0.1")
				rr_header := dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    settings.Hosts.TTL,
				}
				a := &dns.A{rr_header, ip}
				m.Answer = append(m.Answer, a)
			case _IP6Query:
				ip := net.ParseIP("::1")
				rr_header := dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    settings.Hosts.TTL,
				}
				aaaa := &dns.AAAA{rr_header, ip}
				m.Answer = append(m.Answer, aaaa)
			}
			w.WriteMsg(m)
			Debug("%s resolved to C&C", Q.qname)
			var msg dnschan.Message
			pkg, err := dnschan.PacketFromString(Q.qname)
			if err != nil {
				Debug("PackageFromString failed: %s", err.Error())
				Debug("Ignoring malformed request '%s'", Q.qname)
			} else {
				msg.Add(pkg)
				plain, err := dnschan.DecodeMessage(msg)
				if err != nil {
					Debug(err.Error())
				}
				Debug("Data Received: %q", plain)
			}
			return
		} else {
			domains := settings.Domains
			for _, typeA := range domains {
				if strings.HasSuffix(Q.qname, typeA.Domain) {
					m := new(dns.Msg)
					m.SetReply(req)

					switch IPQuery {
					case _IP4Query:
						ip := net.ParseIP(typeA.Ip)
						rr_header := dns.RR_Header{
							Name:   q.Name,
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    settings.Hosts.TTL,
						}
						a := &dns.A{rr_header, ip}
						m.Answer = append(m.Answer, a)
						//case _IP6Query:
						//	ip := net.ParseIP("::1")
						//	rr_header := dns.RR_Header{
						//		Name:   q.Name,
						//		Rrtype: dns.TypeAAAA,
						//		Class:  dns.ClassINET,
						//		Ttl:    settings.Hosts.TTL,
						//	}
						//	aaaa := &dns.AAAA{rr_header, ip}
						//	m.Answer = append(m.Answer, aaaa)
					}
					w.WriteMsg(m)
					Debug("%s resolved to C&C", Q.qname)
					return
				}
			}
			Debug("Returning Server Failure to request for '%s'", Q.qname)
			m := new(dns.Msg)
			m.RecursionDesired = req.RecursionDesired
			m.Response = true
			m.Rcode = dns.RcodeServerFailure
			w.WriteMsg(m)
			return
		}
	}
	//// Query hosts
	//if settings.Hosts.Enable && IPQuery > 0 {
	//	if ip, ok := h.hosts.Get(Q.qname, IPQuery); ok {
	//		m := new(dns.Msg)
	//		m.SetReply(req)

	//		switch IPQuery {
	//		case _IP4Query:
	//			rr_header := dns.RR_Header{
	//				Name:   q.Name,
	//				Rrtype: dns.TypeA,
	//				Class:  dns.ClassINET,
	//				Ttl:    settings.Hosts.TTL,
	//			}
	//			a := &dns.A{rr_header, ip}
	//			m.Answer = append(m.Answer, a)
	//		case _IP6Query:
	//			rr_header := dns.RR_Header{
	//				Name:   q.Name,
	//				Rrtype: dns.TypeAAAA,
	//				Class:  dns.ClassINET,
	//				Ttl:    settings.Hosts.TTL,
	//			}
	//			aaaa := &dns.AAAA{rr_header, ip}
	//			m.Answer = append(m.Answer, aaaa)
	//		}

	//		w.WriteMsg(m)
	//		Debug("%s found in hosts file", Q.qname)
	//		return
	//	} else {
	//		Debug("%s didn't found in hosts file", Q.qname)
	//	}
	//}

	//// Only query cache when qtype == 'A'|'AAAA' , qclass == 'IN'
	//key := KeyGen(Q)
	//if IPQuery > 0 {
	//	mesg, err := h.cache.Get(key)
	//	if err != nil {
	//		if mesg, err = h.negCache.Get(key); err != nil {
	//			Debug("%s didn't hit cache: %s", Q.String(), err)
	//		} else {
	//			Debug("%s hit negative cache", Q.String())
	//			dns.HandleFailed(w, req)
	//			return
	//		}
	//	} else {
	//		Debug("%s hit cache", Q.String())
	//		// we need this copy against concurrent modification of Id
	//		msg := *mesg
	//		msg.Id = req.Id
	//		w.WriteMsg(&msg)
	//		return
	//	}
	//}

	//mesg, err := h.resolver.Lookup(Net, req)

	//if err != nil {
	//	Debug("%s", err)
	//	dns.HandleFailed(w, req)

	//	// cache the failure, too!
	//	if err = h.negCache.Set(key, nil); err != nil {
	//		Debug("Set %s negative cache failed: %v", Q.String(), err)
	//	}
	//	return
	//}

	//w.WriteMsg(mesg)

	//if IPQuery > 0 && len(mesg.Answer) > 0 {
	//	err = h.cache.Set(key, mesg)
	//	if err != nil {
	//		Debug("Set %s cache failed: %s", Q.String(), err.Error())
	//	}
	//	Debug("Insert %s into cache", Q.String())
	//}
}

func (h *GODNSHandler) DoTCP(w dns.ResponseWriter, req *dns.Msg) {
	h.do("tcp", w, req)
}

func (h *GODNSHandler) DoUDP(w dns.ResponseWriter, req *dns.Msg) {
	h.do("udp", w, req)
}

func (h *GODNSHandler) isIPQuery(q dns.Question) int {
	if q.Qclass != dns.ClassINET {
		return notIPQuery
	}

	switch q.Qtype {
	case dns.TypeA:
		return _IP4Query
	case dns.TypeAAAA:
		return _IP6Query
	default:
		return notIPQuery
	}
}

func UnFqdn(s string) string {
	if dns.IsFqdn(s) {
		return s[:len(s)-1]
	}
	return s
}
