package socket

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

type Socket struct {
	FileDescriptor int
	IsOpen         bool
}

func (socket Socket) Read(bytes []byte) (int, error) {
	if len(bytes) == 0 {
		return 0, nil
	}
	numBytesRead, err :=
		syscall.Read(socket.FileDescriptor, bytes)
	if err != nil {
		numBytesRead = 0
	}
	return numBytesRead, err
}

func (socket Socket) Write(bytes []byte) (int, error) {
	numBytesWritten, err :=
		syscall.Write(socket.FileDescriptor, bytes)
	if err != nil {
		numBytesWritten = 0
	}
	return numBytesWritten, err
}

func (socket *Socket) Close() error {
	if socket.IsOpen {
		err := syscall.Close(socket.FileDescriptor)
		if err == nil {
			socket.IsOpen = false
			log.Println("Closed socket", socket)
		}
		return err
	}
	return nil
}

func (socket *Socket) String() string {
	return strconv.Itoa(socket.FileDescriptor)
}

func FromFileDescriptor(fileDescriptor int) *Socket {
	return &Socket{FileDescriptor: fileDescriptor, IsOpen: true}
}

func Listen(ip string, port int) (*Socket, error) {
	socket := &Socket{}

	/*
	Register SIGTERM handler to ensure socket closing.
	*/
	signalChannel := make(chan os.Signal)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		socket.Close()
		os.Exit(0)
	}()

	/*
	Create socket file descriptor.

	- AF_INET = ARPA Internet protocols (IP)
	- SOCK_STREAM = sequenced, reliable, two-way connection based byte streams

	See https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/socket.2.html
	*/
	socketFileDescriptor, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket file descriptor (%v)", err)
	}
	socket.IsOpen = true
	socket.FileDescriptor = socketFileDescriptor
	log.Print("Created new socket ", socket)

	/*
	Set SO_REUSEADDR so I can quickly restart the server but potentially dangerous in production.
	*/
	err = syscall.SetsockoptInt(socket.FileDescriptor, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		return nil, fmt.Errorf("failed set SO_REUSEADDR (%v)", err)
	}

	/*
	Bind the Socket to a port
	*/
	socketAddress := &syscall.SockaddrInet4{Port: port}
	copy(socketAddress.Addr[:], net.ParseIP(ip))
	if err = syscall.Bind(socket.FileDescriptor, socketAddress); err != nil {
		return nil, fmt.Errorf("failed to bind socket (%v)", err)
	}
	log.Print("Bound socket ", socket, " on ", ip, ":", port)

	/*
	Listen for incoming connections.

	See https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/listen.2.html
	*/
	if err = syscall.Listen(socket.FileDescriptor, syscall.SOMAXCONN); err != nil {
		return nil, fmt.Errorf("failed to listen on socket (%v)", err)
	}
	log.Print("Listening on socket ", socket)

	return socket, nil
}
