package tftp

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"sync"
	"time"
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
	requestLog  *log.Logger
}

// WrappedPacket stores the destination address and a serialized tftp packet
type WrappedPacket struct {
	data []byte
	addr net.Addr
}

// NewServer constructs a tftp server with the supplied connection and storage
func NewServer(conn net.PacketConn, store Storage, requestLogWriter io.Writer) *server {
	chanMap := make(map[string]chan []byte)
	return &server{conn: conn, store: store, chanMap: chanMap, requestLog: log.New(requestLogWriter, "", log.LstdFlags)}
}

func (s *server) readConnection(ch chan WrappedPacket) {
	for {
		var recvBuffer [1024]byte
		read, dest, err := s.conn.ReadFrom(recvBuffer[:])
		fmt.Println("Received a packet from ", dest)
		if err != nil {
			return
		}

		packet := WrappedPacket{addr: dest, data: recvBuffer[0:read]}
		ch <- packet
	}
}

func (s *server) sendConnection(ch chan WrappedPacket) {
	for {
		packet := <-ch
		s.conn.WriteTo(packet.data, packet.addr)
	}
}

func (s *server) Start() {
	netChannel := make(chan WrappedPacket, 1024)
	recvChannel := make(chan WrappedPacket, 1024)
	go s.readConnection(recvChannel)
	go s.sendConnection(netChannel)
	for {
		packet := <-recvChannel
		s.chanMapLock.RLock()
		if handlerChannel, ok := s.chanMap[packet.addr.String()]; !ok {
			s.chanMapLock.RUnlock()
			ch := make(chan []byte)
			go func() {
				s.handleClient(ch, packet.addr, netChannel)
			}()
			s.chanMapLock.Lock()
			s.chanMap[packet.addr.String()] = ch
			handlerChannel = s.chanMap[packet.addr.String()]
			handlerChannel <- packet.data
			s.chanMapLock.Unlock()

		} else {
			handlerChannel <- packet.data
			s.chanMapLock.RUnlock()
		}
	}
}

func (s *server) handleClient(ch chan []byte, addr net.Addr, sendChannel chan WrappedPacket) {
	raw := <-ch
	packet, err := ParsePacket(raw)
	if err != nil {
		fmt.Println("Failed to parse tftp packet")
	}

	switch p := packet.(type) {
	case *PacketRequest:
		if p.Op == OpRRQ {
			s.handleReadRequest(packet.(*PacketRequest), ch, addr, sendChannel)
		} else {
			s.handleWriteRequest(packet.(*PacketRequest), ch, addr, sendChannel)
		}
	case *PacketData:
		s.requestLog.Println("Error: Unexpected data packet from ", addr)
	case *PacketError:
		s.requestLog.Printf("Error: Error packet from %v. Code=%v Message=%s\n", addr, p.Code, p.Msg)
	case *PacketAck:
		s.requestLog.Printf("Error: Unexpected ack packet from %v\n", addr)
	default:
		fmt.Println("Error: Unexpected packet type.")
	}
	s.chanMapLock.Lock()
	delete(s.chanMap, addr.String())
	s.chanMapLock.Unlock()
	return
}

func (s *server) handleReadRequest(packet *PacketRequest, ch chan []byte, addr net.Addr, sendChannel chan WrappedPacket) {
	s.requestLog.Printf("Info: Read request for %s from %s\n", packet.Filename, addr)
	file, present := s.store.Get(packet.Filename)
	if !present {
		ret := PacketError{Code: uint16(FileNotFound), Msg: "No such file"}
		buf := ret.Serialize()
		p := WrappedPacket{buf, addr}
		sendChannel <- p
		return
	}

	i := 0
	block := uint16(0)
	for ; i < len(file); i += chunkSize {
		limit := int(math.Min(float64(i+chunkSize), float64(len(file))))
		buf := file[i:limit]
		block = block + 1
		ret := PacketData{BlockNum: block, Data: buf}
		p := WrappedPacket{ret.Serialize(), addr}
		sendChannel <- p

		err := readAck(block, ch)
		if err != nil {
			return
		}
	}
}

func (s *server) handleWriteRequest(packet *PacketRequest, ch chan []byte, addr net.Addr, sendChannel chan WrappedPacket) {
	s.requestLog.Printf("Info: Write request for %s from %s\n", packet.Filename, addr)
	_, exists := s.store.Get(packet.Filename)
	if exists {
		ret := PacketError{Code: uint16(FileAlreadyExists), Msg: "File Already Exists"}
		buf := ret.Serialize()
		p := WrappedPacket{buf, addr}
		sendChannel <- p
		return
	}

	sendAck(0, addr, sendChannel)

	fileBuf := make([]byte, 0)
	for {
		buf := <-ch
		chunk, err := ParsePacket(buf)
		if err != nil {
			return
		}
		switch p := chunk.(type) {
		case *PacketData:
			sendAck(p.BlockNum, addr, sendChannel)
			fileBuf = append(fileBuf, p.Data...)
			if len(p.Data) != chunkSize {
				s.store.Store(packet.Filename, fileBuf)
				return
			}
		default:
			return
		}
	}
}

func sendAck(blockNum uint16, addr net.Addr, sendChannel chan WrappedPacket) error {
	ack := PacketAck{BlockNum: blockNum}
	p := WrappedPacket{ack.Serialize(), addr}
	sendChannel <- p
	return nil
}

func readAck(blockNum uint16, ch chan []byte) error {
	select {
	case buf := <-ch:
		packet, err := ParsePacket(buf)
		if err != nil {
			return err
		}
		switch p := packet.(type) {
		case *PacketAck:
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
