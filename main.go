package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

type Client struct {
	conn net.Conn
	new  bool
}

var clients = map[uint64]Client{}

type InputStream struct {
	pipe      *os.File
	moov      []byte
	timeStamp time.Time
}

func (stream *InputStream) readStream() error {
	pipe := stream.pipe

	// Each MP4 Fragment must start with MP4 header 4B + 4B
	atomHeader := make([]byte, 8)

	for {
		// Blocking Read operation - Read exactly 8 Bytes
		if _, err := io.ReadFull(pipe, atomHeader); err != nil {
			return fmt.Errorf("Error reading atom header: %v\n", err)
		}
		// Parse 4B Length + 4B Type
		atomSize := binary.BigEndian.Uint32(atomHeader[:4])
		atomType := string(atomHeader[4:8])
		if atomSize < 8 {
			return fmt.Errorf("Invalid atom size %d for atom type %s\n", atomSize, atomType)
		}
		// Test if cursor is seeking correctly
		// fmt.Println(atomType, atomSize)

		// TODO: proper initialization
		atomData := make([]byte, 0)

		// if atomType == "moov" || atomType == "moof" {
		if atomType != "mdat" {
			// by specs, atom size includes header, hence each atom is minimum 8 Bytes
			atomData = make([]byte, atomSize-8)
			if _, err := io.ReadFull(pipe, atomData); err != nil {
				return fmt.Errorf("Error reading atom data: %v\n", err)
			}
		}

		switch atomType {

		// moov gets stored as a byte slice
		case "moov":
			fullAtom := bytes.NewBuffer(append(atomHeader, atomData...))
			stream.moov = fullAtom.Bytes()
			stream.timeStamp = time.Now()
			fmt.Println("Received moov atom at", stream.timeStamp)
			break

		// moof gets parsed for identifying keyframes
		case "moof":
			// fullAtom := bytes.NewBuffer(append(atomHeader, atomData...))
			fullAtom := append(atomHeader, atomData...)
			flags := getTrafAtom(fullAtom)

			// normally distribute moof atom to whoever has already been playing
			sockets := make([]io.Writer, 0)
			for _, val := range clients {
				if val.new == false {
					sockets = append(sockets, val.conn)
				}
			}
			broadcast := io.MultiWriter(sockets...)
			broadcast.Write(fullAtom)

			// create MultiWriter for Broadcasting to players waiting for keyframe
			if flags == 0xa05 {
				// fmt.Print("keyframe ", extractSequenceNumber(fullAtom.Bytes()), clients, "\r")
				sockets := make([]io.Writer, 0)
				for id, val := range clients {
					if val.new == true {
						sockets = append(sockets, val.conn)
						// TODO: how to properly edit instances inside map
						instance := clients[id] // next they will receive mdat normally
						instance.new = false
						clients[id] = instance
					}
				}
				broadcast := io.MultiWriter(sockets...)
				broadcast.Write(fullAtom)
			}
			break

		case "mdat":
			sockets := make([]io.Writer, 0)
			for _, val := range clients {
				if val.new == false {
					sockets = append(sockets, val.conn)
				}
			}
			broadcast := io.MultiWriter(sockets...)
			broadcast.Write(atomHeader)
			io.CopyN(broadcast, pipe, int64(atomSize-8))
			break

		// other atoms at root level are ignored
		default:
			fmt.Println("Discarding atom", atomType)
		}
	}
}

func main() {

	pipe, err := os.OpenFile(os.Args[1], os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		panic(err)
	}
	defer pipe.Close()

	stream := InputStream{pipe: pipe}
	go stream.readStream()

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
	// defer conn.Close()
	// TODO: manage close

	clients[uint64(time.Now().UnixNano())] = Client{conn: conn, new: true}

	fmt.Println("new conn", conn.RemoteAddr())

	conn.Write(stream.moov)

}
