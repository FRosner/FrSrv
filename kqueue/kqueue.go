package kqueue

import (
	"FrSrv/socket"
	"fmt"
	"log"
	"syscall"
)

type Kqueue struct {
	FileDescriptor int
}

func FromSocket(s *socket.Socket) (*Kqueue, error) {
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

	return &Kqueue{FileDescriptor: kQueue}, nil
}
