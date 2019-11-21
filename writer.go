package main

import (
	"github.com/friendsofgo/errors"
	"golang.org/x/sys/unix"
)

// RawSocket is a raw IP socket
type RawSocket int

// NewSocket builds a raw IP socket
func NewSocket() (RawSocket, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_RAW)
	if err != nil || fd < 0 {
		return 0, errors.Wrap(err, "Failed to create socket")
	}
	err = unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_HDRINCL, 1)
	if err != nil {
		unix.Close(fd)
		return 0, errors.Wrap(err, "Failed to enable IP_HDRINCL")
	}
	return RawSocket(fd), nil
}

// Write flushes an IP frame through the raw socket
func (s RawSocket) Write(b []byte) (int, error) {
	// just use an empty IPv4 sockaddr for Sendto.
	// the kernel will route the packet based on the IP header
	var emptyAddr unix.SockaddrInet4
	err := unix.Sendto(int(s), b, 0, &emptyAddr)
	if err != nil {
		return 0, errors.Wrap(err, "Failed to flush IPv4 frame")
	}
	return len(b), nil
}

// Close implements io.Closer
func (s RawSocket) Close() {
	unix.Close(int(s))
}
