package kqueue

import (
	"FrSrv/socket"
	"fmt"
	"log"
	"syscall"
)

type EventLoop struct {
	KqueueFileDescriptor int
	SocketFileDescriptor int
}

type Handler = func(s *socket.Socket)

func NewEventLoop(s *socket.Socket) (*EventLoop, error) {
	/*
	Create new new kernel event queue

	See https://www.freebsd.org/cgi/man.cgi?query=kqueue&sektion=2
	*/
	kQueue, err := syscall.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("failed to create kqueue file descriptor (%v)", err)
	}
	log.Print("Created kqueue ", kQueue)

	/*
	Specify event we want to monitor.

	- EVFILT_READ -> receive only events when there is data to read on the Socket
	- EV_ADD | EV_ENABLE -> add event and enable it

	See https://www.freebsd.org/cgi/man.cgi?query=kqueue&sektion=2
	*/
	changeEvent := syscall.Kevent_t{
		Ident:  uint64(s.FileDescriptor),
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
		return nil, fmt.Errorf("failed to register change event (%v)", err)
	}

	return &EventLoop{KqueueFileDescriptor: kQueue, SocketFileDescriptor: s.FileDescriptor}, nil
}

func (eventLoop *EventLoop) Handle(handler Handler) {
	/*
	Event loop, checking the kernel queue for new events and executing handlers.
	*/
	for {
		/*
		Then, we query the queue for pending events, leaving the second argument empty.
		*/
		log.Println("Polling for new events...")
		newEvents := make([]syscall.Kevent_t, 10)
		numNewEvents, err := syscall.Kevent(eventLoop.KqueueFileDescriptor, nil, newEvents, nil)
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
			} else if eventFileDescriptor == eventLoop.SocketFileDescriptor {
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
				socketEventRegistered, err := syscall.Kevent(eventLoop.KqueueFileDescriptor, []syscall.Kevent_t{socketEvent}, nil, nil)
				if err != nil || socketEventRegistered == -1 {
					log.Print("Failed to register Socket event:", err)
					continue
				}
			} else if currentEvent.Filter&syscall.EVFILT_READ != 0 {
				clientSocket := socket.FromFileDescriptor(int(eventFileDescriptor))
				handler(clientSocket)
			}
			// Ignore any other events
		}
	}
}
