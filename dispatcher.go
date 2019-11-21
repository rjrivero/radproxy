package main

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Track number of read errors
	udpReadError = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "udp_read_errors",
			Help: "Count of read errors",
		},
	)
	// Track the duration of requests
	dispatchDuration = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name: "dispatch_duration_seconds",
			Help: "Count of UDP requests",
		},
	)
	// Track number of read errors
	selectionError = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dispatch_selector_errors",
			Help: "Count of errors in filtering logic",
		},
	)
	// Track number of selection rejects
	selectionReject = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dispatch_selector_rejects",
			Help: "Count of filtering logic rejects",
		},
	)
	// Track number of write errors
	writeError = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dispatch_writer_errors",
			Help: "Count of writer errors",
		},
	)
)

func init() {
	prometheus.MustRegister(udpReadError)
	prometheus.MustRegister(dispatchDuration)
	prometheus.MustRegister(selectionError)
	prometheus.MustRegister(selectionReject)
	prometheus.MustRegister(writeError)
}

// PacketConn is a subset of net.PacketConn, with just the functions we need
type PacketConn interface {
	// ReadFrom reads a packet from the connection,
	// copying the payload into p. It returns the number of
	// bytes copied into p and the return address that
	// was on the packet.
	// It returns the number of bytes read (0 <= n <= len(p))
	// and any error encountered. Callers should always process
	// the n > 0 bytes returned before considering the error err.
	// ReadFrom can be made to time out and return
	// an Error with Timeout() == true after a fixed time limit;
	// see SetDeadline and SetReadDeadline.
	ReadFromUDP(p []byte) (n int, addr *net.UDPAddr, err error)
}

type task struct {
	n     int // bytes read
	sAddr net.UDPAddr
	buf   *Buffer
	t     *prometheus.Timer
}

// Selector is an interface to select radius frames to forward
type Selector interface {
	// Select to run for every packet. Return true if frame must be forwarded.
	Select(sAddr net.IP, payload []byte) (bool, error)
}

// Dispatcher waits for requests on the provided Src address,
// and forwards them to the provided Dst address.
type Dispatcher struct {
	// Destination UDP Addresses
	dst net.UDPAddr
	// Selector to run for every packet. Return true if frame must be forwarded.
	// Selector should be concurrency-safe.
	sel Selector
}

// Pipe reads from the PacketConn and writes to the Writer.
// It uses decoupled goroutines so that reader does not block as
// long as there is space in the buffer.
func (d *Dispatcher) pipe(conn PacketConn, sink io.Writer, buffer []Buffer) {
	freeList := make(chan *Buffer, len(buffer))
	taskList := make(chan task, len(buffer))
	for i := range buffer {
		freeList <- &buffer[i]
		// Spawn a task per buffer item
		go func() {
			for task := range taskList {
				t := time.Now()
				valid, err := d.sel.Select(task.sAddr.IP, task.buf.Slice()[0:task.n])
				switch {
				case err != nil:
					// TODO: Log Error
					selectionError.Inc()
					// Forward the frame if we couldn't decide
					fallthrough
				case valid:
					payload := task.buf.SpoofUDP(task.sAddr, d.dst, task.n)
					if _, err = sink.Write(payload); err != nil {
						// TODO: Log Error
						writeError.Inc()
					}
				default:
					selectionReject.Inc()
				}
				freeList <- task.buf
				dispatchDuration.Observe(time.Now().Sub(t).Seconds())
			}
		}()
	}
	// Keep reading while there are free slots
	for b := range freeList {
		n, sAddr, err := conn.ReadFromUDP(b.Slice())
		if err != nil {
			// TODO: Log error
			udpReadError.Inc()
		} else {
			taskList <- task{n: n, sAddr: *sAddr, buf: b}
		}
	}
	close(taskList)
}

// Listen on the PacketConn, and forward frames to the writers.
// The maximum number of simultaneous on-flight transactions is
// len(writers) * writerQueue.
func (d *Dispatcher) Listen(conn PacketConn, writers []io.Writer, writerQueue int) {
	wg := sync.WaitGroup{}
	for _, writer := range writers {
		wg.Add(1)
		go func(writer io.Writer) {
			defer wg.Done()
			d.pipe(conn, writer, make([]Buffer, writerQueue))
		}(writer)
	}
	wg.Wait()
}

// NewDispatcher builds a new Dispatcher
func NewDispatcher(dst net.UDPAddr, sel Selector) *Dispatcher {
	return &Dispatcher{dst: dst, sel: sel}
}
