package main

import (
	"FrSrv/kqueue"
	"FrSrv/socket"
	"bufio"
	"log"
	"os"
	"strings"
	"syscall"
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
	kQueue, err := kqueue.FromSocket(s)
	if err != nil {
		log.Println("Failed to create kqueue:", err)
		os.Exit(1)
	}

	log.Println("Server started. Waiting for incoming connections. ^C to exit.")
	/*
	Event loop, checking the kernel queue for new events and executing handlers.
	*/
	for {
		/*
		Then, we query the queue for pending events, leaving the second argument empty.
		*/
		log.Println("Polling for new events...")
		newEvents := make([]syscall.Kevent_t, 10)
		numNewEvents, err := syscall.Kevent(kQueue.FileDescriptor, nil, newEvents, nil)
		if err != nil {
			/*
			We sometimes get syscall.Errno == 0x4 (EINTR) but that's ok it seems. Just keep polling.
			See https://reviews.llvm.org/D42206
			*/
			continue
		}

		for i := 0; i < numNewEvents; i++ {
			currentEvent := newEvents[i]
			eventFileDescriptor := int(currentEvent.Ident)

			if currentEvent.Flags&syscall.EV_EOF != 0 {
				/*
				Handle client closing the connection. Closing the event file descriptor removes it from the queue.
				*/
				log.Println("Client disconnected.")
				syscall.Close(eventFileDescriptor)
			} else if eventFileDescriptor == s.FileDescriptor {
				/*
				Accept incoming connection.
				*/
				socketConnection, _, err := syscall.Accept(eventFileDescriptor)
				if err != nil {
					log.Println("Failed to create Socket for connecting to client:", err)
					continue
				}
				log.Print("Accepted new connection ", socketConnection, " from ", eventFileDescriptor)

				/*
				Watch for data coming in through the new connection.
				*/
				socketEvent := syscall.Kevent_t{
					Ident:  uint64(socketConnection),
					Filter: syscall.EVFILT_READ,
					Flags:  syscall.EV_ADD,
					Fflags: 0,
					Data:   0,
					Udata:  nil,
				}
				socketEventRegistered, err := syscall.Kevent(kQueue.FileDescriptor, []syscall.Kevent_t{socketEvent}, nil, nil)
				if err != nil || socketEventRegistered == -1 {
					log.Print("Failed to register Socket event:", err)
					continue
				}
			} else if currentEvent.Filter&syscall.EVFILT_READ != 0 {
				/*
				Echo incoming data until empty line is received.
				*/
				clientSocket := socket.FromFileDescriptor(int(eventFileDescriptor))
				reader := bufio.NewReader(clientSocket)
				for {
					line, err := reader.ReadString('\n')
					if err != nil || strings.TrimSpace(line) == "" {
						break
					}
					log.Print("Read on ", eventFileDescriptor, ": ", line)
					clientSocket.Write([]byte(line))
				}
				clientSocket.Close()
			}
			// Ignore any other events
		}
	}
}
