package main

import (
	"log"
	"net"
	"time"
)

func main() {
	c,err := net.Dial("tcp","127.0.0.1:8080")
	if err!= nil {
		log.Fatalln(err)
	}
	defer c.Close()
	_,_ = c.Write([]byte("hello world\n"))
	log.Println("send msg")
	time.Sleep(time.Second*5)
	log.Println("client closed")
}