package main

import (
	"errors"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"igneous.io/tftp"
)

// Error codes
type tftpError uint16

// TFTP Error codes, only FileNotFound and FileAlreadyExists are used
const (
	FileNotFound      tftpError = 1
	AccessViolation   tftpError = 2
	DiskFull          tftpError = 3
	IllegalOperation  tftpError = 4
	UnknownTransferID tftpError = 5
	FileAlreadyExists tftpError = 6
	NoSuchUser        tftpError = 7
)

const chunkSize = 512
const timeoutSeconds = 30

type server struct {
	conn        net.PacketConn
	store       Storage
	chanMap     map[string]chan []byte
	chanMapLock sync.RWMutex
}

// NewServer constructs a tftp server with the supplied connection and storage
func NewServer(conn net.PacketConn, store Storage) *server {
	chanMap := make(map[string]chan []byte)
	return &server{conn: conn, store: store, chanMap: chanMap}
}

func (s *server) start() {
	for {
		var recvBuffer [1024]byte
		read, dest, err := s.conn.ReadFrom(recvBuffer[:])
		if err != nil {
			return
		}

		s.chanMapLock.RLock()
		if handlerChannel, ok := s.chanMap[dest.String()]; !ok {
			s.chanMapLock.RUnlock()
			ch := make(chan []byte)
			go func() {
				s.handleClient(ch, dest, s.conn)
			}()
			s.chanMapLock.Lock()
			fmt.Println("test")
			s.chanMap[dest.String()] = ch
			handlerChannel = s.chanMap[dest.String()]
			handlerChannel <- recvBuffer[0:read]
			s.chanMapLock.Unlock()

		} else {
			handlerChannel <- recvBuffer[0:read]
			s.chanMapLock.RUnlock()
		}

	}
}

func (s *server) addClient(ch chan []byte, addr net.Addr) {

}

func removeClient(addr net.Addr) {

}

func (s *server) handleClient(ch chan []byte, addr net.Addr, conn net.PacketConn) {
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
						s.handleReadRequest(packet.(*tftp.PacketRequest), ch, addr, conn)
						fmt.Println("Done")
					} else {
						fmt.Println("Receiving file...")
						s.handleWriteRequest(packet.(*tftp.PacketRequest), ch, addr, conn)
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
		case <-time.After(timeoutSeconds * time.Second):
			fmt.Println("Closing client", addr)
			s.chanMapLock.Lock()
			delete(s.chanMap, addr.String())
			s.chanMapLock.Unlock()
			return
		}
	}
}

func (s *server) handleReadRequest(packet *tftp.PacketRequest, ch chan []byte, addr net.Addr, conn net.PacketConn) {
	file, present := s.store.Get(packet.Filename)
	if !present {
		ret := tftp.PacketError{Code: uint16(FileNotFound), Msg: "No such file"}
		buf := ret.Serialize()
		_, err := conn.WriteTo(buf, addr)
		if err != nil {
			// log it, don't care
		}
		return
	}

	i := 0
	block := uint16(0)
	fmt.Println("Sending bytes", len(file))
	for ; i < len(file); i += chunkSize {
		fmt.Println("Sending block", block)
		limit := int(math.Min(float64(i+chunkSize), float64(len(file))))
		buf := file[i:limit]
		fmt.Println(len(buf))
		block = block + 1
		ret := tftp.PacketData{BlockNum: block, Data: buf}
		_, err := conn.WriteTo(ret.Serialize(), addr)
		if err != nil {
			fmt.Println("Failed to send")
			// Ignore errors when sending
			return
		}

		err = readAck(block, ch)
		if err != nil {
			fmt.Println("Failed ack")
			return
		}
	}
}

func (s *server) handleWriteRequest(packet *tftp.PacketRequest, ch chan []byte, addr net.Addr, conn net.PacketConn) {
	_, exists := s.store.Get(packet.Filename)
	if exists {
		ret := tftp.PacketError{Code: uint16(FileAlreadyExists), Msg: "File Already Exists"}
		buf := ret.Serialize()
		_, err := conn.WriteTo(buf, addr)
		if err != nil {
			// don't care, add logging
		}
		return
	}

	sendAck(0, addr, conn)

	fileBuf := make([]byte, 0)
	for {
		buf := <-ch
		fmt.Println(len(buf))
		chunk, err := tftp.ParsePacket(buf)
		if err != nil {
			return
		}
		switch p := chunk.(type) {
		case *tftp.PacketData:
			sendAck(p.BlockNum, addr, conn)
			fileBuf = append(fileBuf, p.Data...)
			if len(p.Data) != chunkSize {
				s.store.Store(packet.Filename, fileBuf)
				fmt.Println(len(fileBuf))
				return
			}
		default:
			fmt.Println("Got an unknown packet ", p)
		}
	}
}

func sendAck(blockNum uint16, addr net.Addr, conn net.PacketConn) error {
	ack := tftp.PacketAck{BlockNum: blockNum}
	_, err := conn.WriteTo(ack.Serialize(), addr)
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
	case <-time.After(timeoutSeconds * time.Second):
		return errors.New("No ack packet received")
	}
	panic("Shouldn't reach here")
}
