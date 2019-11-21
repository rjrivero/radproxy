package main

import (
	"encoding/binary"
	"net"
)

// MaxPacketSize is the Maximum UDP packet size in bytes
const MaxPacketSize = 4096

const (
	// IPv4 Version Number
	ipv4Version = 0x45
	// IPv4 Header Size
	ipv4HeaderSize = 20
	// udp4HeaderSize is the size of the UDP header
	udp4HeaderSize = 8
	// pseudo4HeaderSize is the size of the pseudoheader used for checksum
	pseudo4HeaderSize = 12
	// protocolUDP4 is the IP protocol number for UDP
	protocolUDP4 = 17
)

type ipv4Header struct {
	VHL   uint8
	TOS   uint8
	Len   uint16
	ID    uint16
	Offs  uint16
	TTL   uint8
	Proto uint8
	Csum  uint16
	Src   []byte
	Dst   []byte
}

// Write header to byte array. Byte slice must have at least ipv4HeaderSize length.
func (h *ipv4Header) Write(b []byte) {
	b[0] = h.VHL
	b[1] = h.TOS
	binary.BigEndian.PutUint16(b[2:4], h.Len)
	binary.BigEndian.PutUint16(b[4:6], h.ID)
	binary.BigEndian.PutUint16(b[6:8], h.Offs)
	b[8] = h.TTL
	b[9] = h.Proto
	binary.BigEndian.PutUint16(b[10:12], h.Csum)
	copy(b[12:16], h.Src)
	copy(b[16:20], h.Dst)
}

// UDP pseudoHeader used for checksumming
type pseudoHeader struct {
	Src, Dst         []byte
	SrcPort, DstPort uint16
	Protocol         byte
	Len              uint16
}

// Write pseudoHeader to byte array. Must have at least pseudo4HeaderSize length.
func (h *pseudoHeader) Write(b []byte) {
	copy(b[0:4], h.Src)
	copy(b[4:8], h.Dst)
	b[8] = 0
	b[9] = h.Protocol
	binary.BigEndian.PutUint16(b[10:12], h.Len)
	udp := b[12:20]
	binary.BigEndian.PutUint16(udp[0:2], uint16(h.SrcPort))
	binary.BigEndian.PutUint16(udp[2:4], uint16(h.DstPort))
	binary.BigEndian.PutUint16(udp[4:6], h.Len)
	udp[6] = 0 // Init checksum to 0
	udp[7] = 0
	binary.BigEndian.PutUint16(udp[6:8], udpChecksum(b))
}

func udpChecksum(b []byte) uint16 {
	sum := uint32(0)
	for ; len(b) >= 2; b = b[2:] {
		sum += uint32(b[0])<<8 | uint32(b[1])
	}
	if len(b) > 0 {
		sum += uint32(b[0]) << 8
	}
	for sum > 0xffff {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	csum := ^uint16(sum)
	/*
	 * From RFC 768:
	 * If the computed checksum is zero, it is transmitted as all ones (the
	 * equivalent in one's complement arithmetic). An all zero transmitted
	 * checksum value means that the transmitter generated no checksum (for
	 * debugging or for higher level protocols that don't care).
	 */
	if csum == 0 {
		csum = 0xffff
	}
	return csum
}

// Buffer is a packet buffer, including pseudoheader and spoofed UDP header
type Buffer [MaxPacketSize]byte

// Returns the part of the buffer where you can read / copy the payload.
func (b *Buffer) Slice() []byte {
	// Skip IP and UDP headers
	return b[(ipv4HeaderSize + udp4HeaderSize):]
}

// SpoofUDP builds a fake UDP datagram with the provided src and dst addresses.
// The payload used will be the first 'payloadSize' bytes of Slice()
func (b *Buffer) SpoofUDP(src, dst net.UDPAddr, payloadSize int) []byte {
	// UDP length includes header
	udpLen := uint16(payloadSize) + udp4HeaderSize
	ipLen := ipv4HeaderSize + udpLen
	srcIP := src.IP.To4()
	dstIP := dst.IP.To4()
	pseudoFrame := b[(ipv4HeaderSize - pseudo4HeaderSize):ipLen]
	ph := pseudoHeader{
		Src:      srcIP,
		Dst:      dstIP,
		Protocol: protocolUDP4,
		SrcPort:  uint16(src.Port),
		DstPort:  uint16(dst.Port),
		Len:      udpLen,
	}
	ph.Write(pseudoFrame)
	// Overwrite pseudoheader with real IP Header
	ipv4Frame := b[0:ipLen]
	ih := ipv4Header{
		VHL:   ipv4Version,
		Len:   ipLen,
		TOS:   0,
		ID:    0x1234,
		Offs:  0,
		TTL:   16,
		Proto: protocolUDP4,
		Src:   srcIP,
		Dst:   dstIP,
	}
	ih.Write(ipv4Frame)
	return ipv4Frame
}
