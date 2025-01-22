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
		// by specs, atom size includes header, hence each atom is minimum 8 Bytes
		if atomSize < 8 {
			return fmt.Errorf("Invalid atom size %d for atom type %s\n", atomSize, atomType)
		}
		// Test if cursor is seeking correctly
		// fmt.Println(atomType, atomSize)

		atomData := make([]byte, atomSize-8)
		if _, err := io.ReadFull(pipe, atomData); err != nil {
			return fmt.Errorf("Error reading atom data: %v\n", err)
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
				for _, client := range clients {
					if client.new == true {
						client.MarkAsPlaying() // after they will receive every moof and mdat
						if _, err := client.conn.Write(fullAtom); err != nil {
							client.Delete()
						}
					}
				}
			}
			break

		case "mdat":
			for _, client := range clients {
				if client.new == false {
					_, err := client.conn.Write(atomHeader)
					_, err2 := client.conn.Write(atomData)
					if err != nil || err2 != nil {
						client.Delete()
					}
				}
			}
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
	new_id := uint64(time.Now().UnixNano())
	clients[new_id] = &Client{id: new_id, conn: conn, new: true}

	fmt.Println("new conn", conn.RemoteAddr())

	conn.Write(stream.moov)

}
