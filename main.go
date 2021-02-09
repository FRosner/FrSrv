package main

import (
	"bufio"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type Socket struct {
	FileDescriptor int
}

func (socket Socket) Read(bytes []byte) (int, error) {
	if len(bytes) == 0 {
		return 0, nil
	}
	numBytesRead, err := syscall.Read(socket.FileDescriptor, bytes)
	if err != nil {
		numBytesRead = 0
	}
	return numBytesRead, err
}

func (socket Socket) Write(bytes []byte) (int, error) {
	numBytesWritten, err := syscall.Write(socket.FileDescriptor, bytes)
	if err != nil {
		numBytesWritten = 0
	}
	return numBytesWritten, err
}

func (socket *Socket) Close() error {
	return syscall.Close(socket.FileDescriptor)
}

var socket *Socket

func main() {
	ip := "127.0.0.1"
	port := 8080

	/*
   	Ensure we close the socket on shutdown
   	*/
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down")
		// TODO can I wait for TIME_WAIT to ensure the socket is indeed reusable?
		if socket != nil {
			socket.Close()
		}
		log.Println("Closed socket", socket)
		os.Exit(0)
	}()

	/*
	Create a Socket.

	- AF_INET = ARPA Internet protocols (IP)
	- SOCK_STREAM = sequenced, reliable, two-way connection based byte streams

	See https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/socket.2.html
	*/
	socketFileDescriptor, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	socket = &Socket{FileDescriptor: socketFileDescriptor}
	if err != nil {
		log.Println("Failed to create Socket:", err)
		os.Exit(1)
	}
	log.Print("Created new socket ", socket)

	/*
	Useful so I can quickly restart the server but potentially dangerous in production.
	*/
	err = syscall.SetsockoptInt(socket.FileDescriptor, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		log.Println("Failed set SO_REUSEADDR:", err)
		os.Exit(1)
	}

	/*
	Bind the Socket to a port
	*/
	socketAddress := &syscall.SockaddrInet4{Port: port}
	copy(socketAddress.Addr[:], net.ParseIP(ip))
	if err = syscall.Bind(socket.FileDescriptor, socketAddress); err != nil {
		log.Println("Failed to bind socket:", err)
		os.Exit(1)
	}
	log.Print("Bound socket ", socket, " on ", ip, ":", port)

	/*
	Listen for incoming connections.

	See https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/listen.2.html
	*/
	if err = syscall.Listen(socket.FileDescriptor, syscall.SOMAXCONN); err != nil {
		log.Println("Failed to listen on Socket:", err)
		os.Exit(1)
	}
	log.Print("Listening on socket ", socket)

	/*
	Create new new kernel event queue

	See https://www.freebsd.org/cgi/man.cgi?query=kqueue&sektion=2
	*/
	kQueue, err := syscall.Kqueue()
	if err != nil {
		log.Println("Failed to create kernel event queue:", err)
		os.Exit(1)
	}
	log.Print("Created kqueue ", kQueue)

	/*
	Specify event we want to monitor.

	- EVFILT_READ -> receive only events when there is data to read on the Socket
	- EV_ADD | EV_ENABLE -> add event and enable it

	See https://www.freebsd.org/cgi/man.cgi?query=kqueue&sektion=2
	*/
	changeEvent := syscall.Kevent_t{
		Ident:  uint64(socket.FileDescriptor),
		Filter: syscall.EVFILT_READ,
		Flags:  syscall.EV_ADD | syscall.EV_ENABLE,
		Fflags: 0,
		Data:   0,
		Udata:  nil,
	}

	/*
	The kevent() system call is used to register events with the queue, and return any pending events to the user.
	First, we register the change event with the queue, leaving the third argument empty.

	See https://www.freebsd.org/cgi/man.cgi?query=kqueue&sektion=2
	*/
	changeEventRegistered, err := syscall.Kevent(kQueue, []syscall.Kevent_t{changeEvent}, nil, nil)
	if err != nil || changeEventRegistered == -1 {
		log.Print("Failed to register changeEvent:", err)
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
		newEvents := make([]syscall.Kevent_t, 10)
		numNewEvents, err := syscall.Kevent(kQueue, nil, newEvents, nil)
		if err != nil {
			/*
			We sometimes get syscall.Errno == 0x4 (EINTR) but that's ok it seems.
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
			} else if eventFileDescriptor == socket.FileDescriptor {
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
				socketEventRegistered, err := syscall.Kevent(kQueue, []syscall.Kevent_t{socketEvent}, nil, nil)
				if err != nil || socketEventRegistered == -1 {
					log.Print("Failed to register Socket event:", err)
					continue
				}
			} else if currentEvent.Filter&syscall.EVFILT_READ != 0 {
				/*
				Echo incoming data until empty line is received.
				*/
				clientSocket := Socket{FileDescriptor: int(eventFileDescriptor)}
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
