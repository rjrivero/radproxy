package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func userSizer(net.IP) (fill, max int) {
	return 5, 10
}

func portSizer(net.IP) (fill, max int) {
	return 5, 10
}

func metrics() {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":8080", nil)
}

func main() {
	lAddr, err := net.ResolveUDPAddr("udp", ":9999")
	if err != nil {
		log.Fatalln(err)
	}
	sConn, err := net.ListenUDP("udp", lAddr)
	if err != nil {
		log.Fatalln(err)
	}
	defer sConn.Close()
	fmt.Println("listening on ", sConn.LocalAddr().String())
	dAddr, err := net.ResolveUDPAddr("udp", "100.1.2.3:8888")
	if err != nil {
		log.Fatalln(err)
	}
	writer, err := NewSocket()
	if err != nil {
		log.Fatalln(err)
	}
	defer writer.Close()
	dispatcher := NewDispatcher(*dAddr, NewSelector(time.Minute, 5, userSizer, portSizer))
	go metrics()
	dispatcher.Listen(sConn, []io.Writer{writer}, 16)
}
