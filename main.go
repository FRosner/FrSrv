package main

import (
	"FrSrv/kqueue"
	"FrSrv/socket"
	"bufio"
	"log"
	"os"
)

func main() {
	s, err := socket.Listen("127.0.0.1", 8080)
	if err != nil {
		log.Println("Failed to create Socket:", err)
		os.Exit(1)
	}

	eventLoop, err := kqueue.NewEventLoop(s)
	if err != nil {
		log.Println("Failed to create kqueue:", err)
		os.Exit(1)
	}

	log.Println("Server started. Waiting for incoming connections. ^C to exit.")
	eventLoop.Handle(func(s *socket.Socket) {
		reader := bufio.NewReader(s)
		line, _ := reader.ReadString('\n')
		log.Print("Read on ", s, ": ", line)
		s.Write([]byte(line))
	})
}
