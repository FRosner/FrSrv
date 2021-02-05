package main

import (
    "bufio"
    "syscall"
    "fmt"
    "os"
    "net"
)

func main() {
    ip := "127.0.0.1"
    port := 8080

    /* Create a socket.
     *
     * - AF_INET = ARPA Internet protocols (IP)
     * - SOCK_STREAM = sequenced, reliable, two-way connection based byte streams
     *
     * See https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/socket.2.html
     */
    socket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
    if err != nil {
        fmt.Println("Failed to create socket:", err)
        os.Exit(1)
    }

    // Bind the socket to a port
    socketAddress := &syscall.SockaddrInet4{Port: port}
    copy(socketAddress.Addr[:], net.ParseIP(ip))
    if err = syscall.Bind(socket, socketAddress); err != nil {
        fmt.Println("Failed to bind socket:", err)
        os.Exit(1)
    }

    /* Listen for incoming connections.
     *
     * See https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/listen.2.html
     */
    if err = syscall.Listen(socket, syscall.SOMAXCONN); err != nil {
        fmt.Println("Failed to listen on socket:", err)
        os.Exit(1)
    }

    fmt.Print("Server started. Waiting for incoming connections. Press any key to exit.")
    input := bufio.NewScanner(os.Stdin)
    input.Scan()
    fmt.Println(input.Text())
}
