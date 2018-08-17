package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"igneous.io/tftp"
)

var files map[string][]byte

func main() {
	port := flag.Int("port", 69, "Port number to listen on")
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%v", *port))
	if err != nil {
		log.Fatal("Unable to resolve listening address: ", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal("Unable to open socket: ", err)
	}
	defer conn.Close()

	f, err := os.Create("tftp_request.log")
	defer f.Close()
	if err != nil {
		log.Fatal("Unable to create request log.")
	}
	store := tftp.NewInMemoryStore()
	server := tftp.NewServer(conn, store, f)

	server.Start()
	fmt.Println("Ready")
}
