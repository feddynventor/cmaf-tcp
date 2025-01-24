package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

var clients = map[uint64]*Client{}

type Client struct {
	id   uint64
	conn net.Conn
	new  bool
}

func (c *Client) MarkAsPlaying() {
	c.new = false
	fmt.Println(c.conn.RemoteAddr(), "is now playing")
}

func (c *Client) Delete() {
	delete(clients, c.id)
	c.conn.Close()
	fmt.Println(c.conn.RemoteAddr(), "disconnected")
}

type InputStream struct {
	data      io.Reader
	moov      []byte
	timeStamp time.Time
}

func atomParser(stream *InputStream, data io.Reader) {
	// Each MP4 Fragment must start with MP4 header 4B + 4B
	atomHeader := make([]byte, 8)
	for {
		if _, err := io.ReadFull(data, atomHeader); err != nil {
			fmt.Println("Error reading atom header:", err)
			return
		}
		atomSize := binary.BigEndian.Uint32(atomHeader[:4])
		atomType := string(atomHeader[4:8])
		// by specs, atom size includes header, hence each atom is minimum 8 Bytes
		if atomSize < 8 {
			fmt.Printf("Invalid atom size %d for atom type %s\n", atomSize, atomType)
			return
		}
		atomData := make([]byte, atomSize-8)
		if _, err := io.ReadFull(data, atomData); err != nil {
			fmt.Println("Error reading atom data:", err)
			return
		}
		switch atomType {
		case "moov":
			fullAtom := append(atomHeader, atomData...)
			stream.moov = fullAtom
			stream.timeStamp = time.Now()
			fmt.Println("Received moov atom at", stream.timeStamp)
			break
		case "moof":
			fullAtom := append(atomHeader, atomData...)
			seq := extractSequenceNumber(fullAtom)
			flags := getTrafAtom(fullAtom)
			fmt.Println("moof", seq, flags)
		case "mdat":
			fmt.Println("mdat", atomSize)
		default:
			fmt.Printf("Ignored atom: %s\n", atomType)
		}
	}
}

func main() {

	namedPipe, err := os.OpenFile(os.Args[1], os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		panic(err)
	}
	defer namedPipe.Close()

	data, pipe := io.Pipe()

	stream := InputStream{data: data}
	parser := io.TeeReader(namedPipe, pipe)

	go atomParser(&stream, parser)

	listener, err := net.Listen("tcp", "0.0.0.0:8080")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	// Concurrent connection handler
	for {
		// Wait for a connection.
		conn, err := listener.Accept()
		if err != nil {
			panic(err) // TODO: to properly manage
		}

		go handleConnection(conn, stream)

	}

}

func handleConnection(conn net.Conn, stream InputStream) {
	new_id := uint64(time.Now().UnixNano())
	clients[new_id] = &Client{id: new_id, conn: conn, new: true}

	fmt.Println("new conn", conn.RemoteAddr())

	conn.Write(stream.moov)

	io.Copy(conn, stream.data)

}
