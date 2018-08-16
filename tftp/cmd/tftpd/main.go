package main

import (
	"fmt"
	"net"
	"os"
)

var files map[string][]byte

func main() {
	fmt.Println("Starting tftpd...")

	addr, err := net.ResolveUDPAddr("udp4", ":69")
	if err != nil {
		fmt.Println("Unable to resolve address")
		os.Exit(1)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("Unable to open socket: ", err)
		os.Exit(1)
	}
	defer conn.Close()

	store := NewInMemoryStore()
	server := NewServer(conn, store)

	server.start()
}
