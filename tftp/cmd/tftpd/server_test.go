package main

import (
	"net"
	"testing"
)

type mockConnection struct {
	addr net.Addr
	recv []byte
	send []byte
}

func TestSmallFile(t *testing.T) {

}

func TestLargeFile(t *testing.T) {

}

func TestFileNotFound(t *testing.T) {

}

func TestDuplicateFile(t *testing.T) {

}

func TestMultipleReaders(t *testing.T) {

}
