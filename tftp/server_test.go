package tftp

import (
	"bytes"
	"fmt"
	"math"
	"net"
	"testing"
	"time"
)

var largeFileData = []byte(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789
abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789`)

type MockConn struct {
}

func (m *MockConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	for {
	}
}

func (m *MockConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	for {
	}
}

func (m *MockConn) Close() error {
	return nil
}

func (m *MockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *MockConn) LocalAddr() net.Addr {
	return nil
}

func (m *MockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type FakeAddr struct{}

func (f *FakeAddr) Network() string {
	return "udp"
}

func (f *FakeAddr) String() string {
	return ":5000"
}

func TestReadChunkedFile(t *testing.T) {
	conn := MockConn{}
	store := NewInMemoryStore()
	store.Store("test.txt", largeFileData)
	server := NewServer(&conn, store)

	addr := new(FakeAddr)
	sender := make(chan []byte)
	reader := make(chan WrappedPacket)

	request := PacketRequest{Op: OpRRQ, Filename: "test.txt", Mode: "netascii"}
	go server.handleReadRequest(&request, sender, addr, reader)

	buf := make([]byte, 0, 512)
	for {
		response := <-reader
		data := new(PacketData)
		if err := data.Parse(response.data); err != nil {
			t.Error("Error while parsing data packet: ", err)
		}

		ack := PacketAck{BlockNum: data.BlockNum}
		sender <- ack.Serialize()

		buf = append(buf, data.Data...)
		if len(data.Data) != chunkSize {
			break
		} else {
			fmt.Println("received: ", len(data.Data))
		}
	}
	if bytes.Compare(largeFileData, buf) != 0 {
		t.Error("Retrieved file is different that original.")
	}
}

func TestWriteChunkedFile(t *testing.T) {
	conn := MockConn{}
	store := NewInMemoryStore()
	server := NewServer(&conn, store)

	addr := new(FakeAddr)
	sender := make(chan []byte)
	reader := make(chan WrappedPacket)

	// Send the write request to the server
	request := PacketRequest{Op: OpWRQ, Filename: "test.txt", Mode: "netascii"}
	go server.handleWriteRequest(&request, sender, addr, reader)

	response := <-reader
	if response.addr.String() != addr.String() {
		t.Error("Ack sent to incorrect address")
	}

	verifyAck(t, response.data, 0)

	i := 0
	block := uint16(0)

	for ; i < len(largeFileData); i += chunkSize {
		limit := int(math.Min(float64(i+chunkSize), float64(len(largeFileData))))
		buf := largeFileData[i:limit]
		block = block + 1
		data := PacketData{BlockNum: block, Data: buf}
		sender <- data.Serialize()

		response = <-reader
		if response.addr.String() != addr.String() {
			t.Error("Ack sent to incorrect address")
		}

		verifyAck(t, response.data, block)
	}

	if file, ok := store.Get("test.txt"); !ok {
		t.Error("File was not written into store")
	} else if bytes.Compare(file, largeFileData) != 0 {
		t.Error("File data is incorrect")
	}
}

func TestReadSmallFile(t *testing.T) {
	smallFileData := []byte("This is a test file")
	conn := MockConn{}
	store := NewInMemoryStore()
	store.Store("smallfile.txt", smallFileData)
	server := NewServer(&conn, store)

	addr := new(FakeAddr)
	sender := make(chan []byte)
	reader := make(chan WrappedPacket)

	request := PacketRequest{Op: OpRRQ, Filename: "smallfile.txt", Mode: "netascii"}
	go server.handleReadRequest(&request, sender, addr, reader)

	response := <-reader
	data := new(PacketData)
	if err := data.Parse(response.data); err != nil {
		t.Error("Error while parsing data packet: ", err)
	}

	if bytes.Compare(smallFileData, data.Data) != 0 {
		t.Error("Retrieved file is different that original.")
	}

}

func TestWriteSmallFile(t *testing.T) {
	smallFileData := []byte("This is a test file")
	conn := MockConn{}
	store := NewInMemoryStore()
	server := NewServer(&conn, store)

	addr := new(FakeAddr)
	sender := make(chan []byte)
	reader := make(chan WrappedPacket)

	// Send the write request to the server
	request := PacketRequest{Op: OpWRQ, Filename: "smallfile.txt", Mode: "netascii"}
	go server.handleWriteRequest(&request, sender, addr, reader)

	response := <-reader
	if response.addr.String() != addr.String() {
		t.Error("Ack sent to incorrect address")
	}

	verifyAck(t, response.data, 0)
	data := PacketData{BlockNum: 1, Data: smallFileData}
	sender <- data.Serialize()

	response = <-reader
	if response.addr.String() != addr.String() {
		t.Error("Ack sent to incorrect address")
	}

	verifyAck(t, response.data, 1)

	if file, ok := store.Get("smallfile.txt"); !ok {
		t.Error("File was not written into store")
	} else if bytes.Compare(file, smallFileData) != 0 {
		t.Error("File data is incorrect")
	}
}

func verifyAck(t *testing.T, data []byte, blockNum uint16) {
	ack := new(PacketAck)
	if err := ack.Parse(data); err != nil {
		t.Error("Error parsing ack packet: ", err)
	}

	if ack.BlockNum != blockNum {
		t.Errorf("Got incorrect block number on ack. Expected: %v Got: %v", blockNum, ack.BlockNum)
	}
}

func TestLargeFile(t *testing.T) {

}

func TestFileNotFound(t *testing.T) {
	conn := MockConn{}
	store := NewInMemoryStore()
	server := NewServer(&conn, store)

	addr := new(FakeAddr)
	sender := make(chan []byte)
	reader := make(chan WrappedPacket)
	request := PacketRequest{Op: OpRRQ, Filename: "smallfile.txt", Mode: "netascii"}
	go server.handleReadRequest(&request, sender, addr, reader)

	response := <-reader

	if response.addr.String() != addr.String() {
		t.Error("Response sent to incorrect address: ", response.addr)
	}

	errorPacket := new(PacketError)
	if err := errorPacket.Parse(response.data); err != nil {
		t.Error("Got an error when parsing error packet: ", err)
	}

	if errorPacket.Code != uint16(FileNotFound) {
		t.Error("Got incorrect error code: ", errorPacket.Code)
	}
}

func TestDuplicateFile(t *testing.T) {
	conn := MockConn{}
	store := NewInMemoryStore()
	store.Store("testfile.txt", []byte("data"))
	server := NewServer(&conn, store)

	addr := new(FakeAddr)
	sender := make(chan []byte)
	reader := make(chan WrappedPacket)
	request := PacketRequest{Op: OpRRQ, Filename: "testfile.txt", Mode: "netascii"}
	go server.handleWriteRequest(&request, sender, addr, reader)

	response := <-reader

	if response.addr.String() != addr.String() {
		t.Error("Response sent to incorrect address: ", response.addr)
	}

	errorPacket := new(PacketError)
	if err := errorPacket.Parse(response.data); err != nil {
		t.Error("Got an error when parsing error packet: ", err)
	}

	if errorPacket.Code != uint16(FileAlreadyExists) {
		t.Error("Got incorrect error code: ", errorPacket.Code)
	}
}
