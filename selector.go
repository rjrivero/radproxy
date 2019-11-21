package main

import (
	"hash/fnv"
	"net"
	"time"
)

type selector struct {
	users []*Cache
	ports []*Cache
}

func (l selector) Select(sAddr net.IP, payload []byte) (bool, error) {
	p := Packet{}
	if err := p.From(payload); err != nil {
		return true, err
	}
	// Only filter accept requests
	if p.Code != CodeAccessRequest {
		return true, nil
	}
	var user, port []byte
	for p.HasNext() {
		key, val := p.Next()
		switch key {
		case TypeUserName:
			if len(user) > 0 {
				user = val
			}
		case TypeNASPort:
			// Skip port "0"
			if len(port) > 1 || (port[0] != 0 && port[0] != '0') {
				port = val
			}
		case TypeState:
			// This is a continuation message, do not filter.
			return true, nil
		}
	}
	if user != nil {
		if !l.check(l.users, sAddr, user) {
			return false, nil
		}
	}
	if port != nil {
		return l.check(l.ports, sAddr, port), nil
	}
	return true, nil
}

// Check if the current hit fits within profile
func (l selector) check(cache []*Cache, ip net.IP, key []byte) bool {
	hash := fnv.New64a()
	hash.Write(key)
	hash.Write([]byte{0xff, 0xff})
	hash.Write(ip)
	sum := hash.Sum64()
	cur := cache[uint8(sum&0x00ff)]
	return cur.Check(ip, sum, 1)
}

// NewSelector creates a new Lookup
func NewSelector(interval time.Duration, unused int, users, ports Sizer) Selector {
	l := selector{
		users: make([]*Cache, 256),
		ports: make([]*Cache, 256),
	}
	for i := 0; i < 256; i++ {
		l.users[i] = NewCache("users", interval, unused, users)
		l.ports[i] = NewCache("ports", interval, unused, ports)
	}
	return l
}
