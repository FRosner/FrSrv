package main

import (
	"FrSrv/kqueue"
	"FrSrv/socket"
	"bufio"
	"log"
	"os"
	"strings"
)

func main() {
	/*
	Create a Socket.

	- AF_INET = ARPA Internet protocols (IP)
	- SOCK_STREAM = sequenced, reliable, two-way connection based byte streams

	See https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/socket.2.html
	*/

	s, err := socket.Listen("127.0.0.1", 8080)
	if err != nil {
		log.Println("Failed to create Socket:", err)
		os.Exit(1)
	}

	/*
	Create new new kernel event queue

	See https://www.freebsd.org/cgi/man.cgi?query=kqueue&sektion=2
	*/
	eventLoop, err := kqueue.NewEventLoop(s)
	if err != nil {
		log.Println("Failed to create kqueue:", err)
		os.Exit(1)
	}

	log.Println("Server started. Waiting for incoming connections. ^C to exit.")
	eventLoop.Handle(func(s *socket.Socket) {
		reader := bufio.NewReader(s)
		for {
			line, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(line) == "" {
				break
			}
			log.Print("Read on ", s, ": ", line)
			s.Write([]byte(line))
		}
		s.Close()
	})
}
