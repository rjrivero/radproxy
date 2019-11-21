package main

import (
	"encoding/binary"

	"github.com/friendsofgo/errors"
)

// Code defines the RADIUS packet type.
type Code int

// Standard RADIUS packet codes.
const (
	CodeAccessRequest      Code = 1
	CodeAccessAccept       Code = 2
	CodeAccessReject       Code = 3
	CodeAccountingRequest  Code = 4
	CodeAccountingResponse Code = 5
	CodeAccessChallenge    Code = 11
	CodeStatusServer       Code = 12
	CodeStatusClient       Code = 13
	CodeDisconnectRequest  Code = 40
	CodeDisconnectACK      Code = 41
	CodeDisconnectNAK      Code = 42
	CodeCoARequest         Code = 43
	CodeCoAACK             Code = 44
	CodeCoANAK             Code = 45
	CodeReserved           Code = 255
)

// Type is the RADIUS attribute type.
type Type int

const (
	TypeUserName     Type = 1
	TypeNASIPAddress Type = 4
	TypeNASPort      Type = 5
	TypeState        Type = 24
)

// TypeInvalid is a Type that can be used to represent an invalid RADIUS
// attribute type.
const TypeInvalid Type = -1

// Packet is a RADIUS packet.
type Packet struct {
	Code          Code
	Identifier    byte
	Authenticator []byte
	cursor        []byte
}

// From fills a Packet struct from the UDP payload of a Radius message
func (p *Packet) From(b []byte) error {
	if len(b) < 20 {
		return errors.New("radius: packet not at least 20 bytes long")
	}
	length := int(binary.BigEndian.Uint16(b[2:4]))
	if length != len(b) {
		return errors.New("radius: invalid packet length")
	}
	p.Code = Code(b[0])
	p.Identifier = b[1]
	p.Authenticator = b[4:20]
	p.cursor = b[20:]
	return nil
}

// HasNext returns true if packet has more attributes
func (p *Packet) HasNext() bool {
	if len(p.cursor) < 2 {
		return false
	}
	length := int(p.cursor[1])
	if length > len(p.cursor) || length < 2 || length > 255 {
		return false
	}
	return true
}

// Next attribute (type and bytes without prefix)
func (p *Packet) Next() (Type, []byte) {
	length := int(p.cursor[1])
	attrib := p.cursor[0:length]
	p.cursor = p.cursor[length:]
	return Type(attrib[0]), attrib[2:]
}
