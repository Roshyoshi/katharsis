//go:build qa

// Command proxy is an HTTP CONNECT proxy for chaos runs whose behavior is
// switched at run time, so a test can cut or stall the game's downloads
// mid-transfer. Point the game at it with HTTPS_PROXY=http://ADDR.
//
//	go build -tags qa -o /tmp/proxy ./qa/chaos/proxy
//	/tmp/proxy -addr 127.0.0.1:9877 -mode /tmp/mode
//
// The mode file holds one word, read for each new tunnel and every 200 ms
// during one:
//
//	pass   forward (the default when the file is missing)
//	hole   accept and never answer: a network that drops packets
//	stall  stop moving bytes on open tunnels, keep them open
//	cut    close every open tunnel, refuse new ones
package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

var modeFile string

func mode() string {
	b, err := os.ReadFile(modeFile)
	if err != nil {
		return "pass"
	}
	return strings.TrimSpace(string(b))
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9877", "listen address")
	flag.StringVar(&modeFile, "mode", "", "file holding the mode")
	flag.Parse()
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s, mode file %s", *addr, modeFile)
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go serve(c)
	}
}

func serve(c net.Conn) {
	defer c.Close()
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	m := mode()
	log.Printf("%s %s (mode %s)", req.Method, req.Host, m)
	switch m {
	case "hole":
		select {} // hold the connection open, say nothing
	case "cut":
		return
	}
	if req.Method != http.MethodConnect {
		_, _ = io.WriteString(c, "HTTP/1.1 405 Method Not Allowed\r\n\r\n")
		return
	}
	up, err := net.DialTimeout("tcp", req.Host, 10*time.Second)
	if err != nil {
		_, _ = io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer up.Close()
	_, _ = io.WriteString(c, "HTTP/1.1 200 Connection established\r\n\r\n")

	var moved atomic.Int64
	done := make(chan struct{}, 2)
	pipe := func(dst io.Writer, src io.Reader) {
		buf := make([]byte, 32<<10)
		for {
			for mode() == "stall" {
				time.Sleep(200 * time.Millisecond)
			}
			n, err := src.Read(buf)
			if n > 0 {
				moved.Add(int64(n))
				if _, werr := dst.Write(buf[:n]); werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}
	go pipe(up, br)
	go pipe(c, up)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-done:
			log.Printf("%s done, %d bytes", req.Host, moved.Load())
			return
		case <-tick.C:
			if mode() == "cut" {
				log.Printf("%s cut after %d bytes", req.Host, moved.Load())
				return
			}
		}
	}
}
