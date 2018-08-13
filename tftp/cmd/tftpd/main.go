package main

import (
	"fmt"
	"net"
	"os"

	"igneous.io/tftp"
)

// TFTP error codes
const (
	ErrFileNotFound   = 1
	AccessViolation   = 2
	DiskFull          = 3
	IllegalOperation  = 4
	UnknownTransferID = 5
	FileAlreadyExists = 6
	NoSuchUser        = 7
)

func main() {
	fmt.Println("Starting")

	addr, err := net.ResolveUDPAddr("udp4", ":69")
	if err != nil {
		fmt.Println("Unable to resolve address")
		os.Exit(1)
	}
	ln, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("Unable to open socket: ", err)
		os.Exit(1)
	}
	defer ln.Close()

	for {
		var buf [512]byte
		_, _, err := ln.ReadFrom(buf[0:])
		if err != nil {
			return
		}
		packet, err := tftp.ParsePacket(buf[0:])
		if err != nil {
			fmt.Println("Invalid TFTP packet")
		}

		switch p := packet.(type) {
		case *tftp.PacketRequest:
			if p.Op == tftp.OpRRQ {
				ch := make(chan tftp.Packet)
				go handleReadRequest(addr, ch, ln)
			} else {
				ch := make(chan tftp.Packet)
				go handleWriteRequest(addr, ch, ln)
			}
		case *tftp.PacketError:
			fmt.Println("Got error packet")
		case *tftp.PacketData:
			fmt.Println("Got data packet")
		case *tftp.PacketAck:
			fmt.Println("Got ACK packet")
		default:
			fmt.Println("Unknown packet type: ", p)
		}

		// response := tftp.PacketError{Code: ErrFileNotFound, Msg: "No such file"}

		// ser := response.Serialize()

		// ln.WriteToUDP(ser, addr.(*net.UDPAddr))
		// if err != nil {
		// 	fmt.Println("Failed to send response")
		// }
	}
	// TODO implement the in-memory tftp server
}

func handleReadRequest(addr *net.UDPAddr, ch chan tftp.Packet, ln *net.UDPConn) {
	fmt.Println("Handling read request")
}

func handleWriteRequest(addr *net.UDPAddr, ch chan tftp.Packet, ln *net.UDPConn) {
	fmt.Println("Handling write request")
}

func handleClient(conn net.Conn) {
	fmt.Println("Handling connection")
}
