package main

import (
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"time"

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

// Chunk size
const (
	ChunkSize = 512
)

// Timeout
const (
	Timeout = 30
)

var files map[string][]byte

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

	chanMap := make(map[string]chan []byte)
	files = make(map[string][]byte)
	for {
		var buf [512]byte
		read, dest, err := ln.ReadFrom(buf[0:])
		if err != nil {
			return
		}

		fmt.Print(dest.String())
		if ch, ok := chanMap[dest.String()]; !ok {
			fmt.Println("Not in map ", dest)
			fmt.Println("map ", chanMap)
			i := make(chan []byte)
			go func() {
				handleClient(i, dest.(*net.UDPAddr), ln)
				delete(chanMap, dest.String())
			}()
			chanMap[dest.String()] = i
			ch = chanMap[dest.String()]
			ch <- buf[0:read]
		} else {
			ch <- buf[0:read]
		}
	}
	// TODO implement the in-memory tftp server
}

func handleClient(ch chan []byte, addr *net.UDPAddr, conn *net.UDPConn) {
	fmt.Println("Handling connection")

	for {
		select {
		case raw := <-ch:
			{
				packet, err := tftp.ParsePacket(raw)
				if err != nil {
					fmt.Println("Failed to parse tftp packet")
				}

				switch p := packet.(type) {
				case *tftp.PacketRequest:
					if p.Op == tftp.OpRRQ {
						fmt.Println("Sending file...")
						handleReadRequest(packet.(*tftp.PacketRequest), ch, addr, conn)
						fmt.Println("Done")
					} else {
						fmt.Println("Receiving file...")
						handleWriteRequest(packet.(*tftp.PacketRequest), ch, addr, conn)
						fmt.Println("Done")
					}
				case *tftp.PacketData:
					fmt.Println("Shouldn't get a data packet here")
				case *tftp.PacketError:
					fmt.Println("Got an error packet")
				case *tftp.PacketAck:
					fmt.Println("Shouldn't get an ack packet here")
				default:
					fmt.Println("Unknown packet type ", packet)
				}
			}
		case <-time.After(Timeout * time.Second):
			return
		}
	}
}

func handleReadRequest(packet *tftp.PacketRequest, ch chan []byte, addr *net.UDPAddr, conn *net.UDPConn) {
	file, present := files[packet.Filename]
	if !present {
		ret := tftp.PacketError{Code: ErrFileNotFound, Msg: "No such file"}
		buf := ret.Serialize()
		conn.WriteToUDP(buf, addr)
		return
	}

	i := 0
	block := uint16(0)
	for ; i < len(file); i += ChunkSize {
		limit := int(math.Min(float64(i+512), float64(len(file))))
		buf := file[i:limit]
		fmt.Println(len(buf))
		block = block + 1
		ret := tftp.PacketData{BlockNum: block, Data: buf}
		_, err := conn.WriteToUDP(ret.Serialize(), addr)
		if err != nil {
			// Ignore errors when sending
			return
		}

		err = readAck(block, ch)
		if err != nil {
			return
		}
	}
}

func handleWriteRequest(packet *tftp.PacketRequest, ch chan []byte, addr *net.UDPAddr, conn *net.UDPConn) {
	if _, present := files[packet.Filename]; present {
		ret := tftp.PacketError{Code: FileAlreadyExists, Msg: "File Already Exists"}
		buf := ret.Serialize()
		conn.WriteToUDP(buf, addr)
		return
	}

	sendAck(0, addr, conn)

	fileBuf := make([]byte, ChunkSize)
	for {
		fmt.Println("Receiving chunk")
		buf := <-ch
		chunk, err := tftp.ParsePacket(buf)
		if err != nil {
			fmt.Println("Error parsing packet")
			return
		}
		switch p := chunk.(type) {
		case *tftp.PacketData:
			fmt.Println("Got a data chunk", p.BlockNum)
			sendAck(p.BlockNum, addr, conn)
			fileBuf = append(fileBuf, p.Data...)
			if len(p.Data) != ChunkSize {
				files[packet.Filename] = buf
				return
			}
		default:
			fmt.Println("Got an unknown packet ", p)
		}
	}
}

func sendAck(blockNum uint16, addr *net.UDPAddr, conn *net.UDPConn) error {
	ack := tftp.PacketAck{BlockNum: blockNum}
	_, err := conn.WriteToUDP(ack.Serialize(), addr)
	return err
}

func readAck(blockNum uint16, ch chan []byte) error {
	select {
	case buf := <-ch:
		packet, err := tftp.ParsePacket(buf)
		if err != nil {
			return err
		}
		switch p := packet.(type) {
		case *tftp.PacketAck:
			if p.BlockNum != blockNum {
				return errors.New("Incorrect block number in ack")
			}
			return nil
		}
	case <-time.After(Timeout * time.Second):
		return errors.New("No ack packet received")
	}
	panic("Shouldn't reach here")
}
