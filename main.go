package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/brucespang/go-tcpinfo"
	"github.com/justincormack/go-memfd"
)

var clients sync.Map

type Client struct {
	conn    net.Conn
	conn_fd *os.File
	new     bool
}

func (c *Client) MarkAsPlaying() {
	c.new = false
	fmt.Println(c.conn.RemoteAddr(), "is now playing")
}

func (c *Client) Delete() {
	clients.Delete(c.conn.RemoteAddr().String())
	c.conn.Close()
	c.conn_fd.Close()
	fmt.Println(c.conn.RemoteAddr(), "disconnected")
}

type Fragment struct {
	moof       []byte
	data       *memfd.Memfd
	keyframe   bool
	timestamp  int64
	byteLength int
}

type InputStream struct {
	moov          []byte
	timeStamp     time.Time
	lastSeqNumber uint32
	fragments     sync.Map
}

func (stream *InputStream) atomParser(data io.Reader) {
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

		if atomType != "mdat" && atomType != "moof" && atomType != "moov" {
			continue
		}
		fullAtom := append(atomHeader, atomData...) // new slice with full atom

		switch atomType {
		case "moov":
			stream.moov = fullAtom // TODO: assert, is this a copy?
			stream.timeStamp = time.Now()
			fmt.Println("Received moov atom at", stream.timeStamp)
			break
		case "moof":
			seq := extractSequenceNumber(fullAtom)
			flags := getTrafAtom(fullAtom)
			stream.lastSeqNumber = seq
			stream.fragments.Store(seq, &Fragment{
				moof:       fullAtom, // slices are passed by reference
				keyframe:   flags == 0xa05,
				byteLength: int(atomSize),
				timestamp:  time.Since(stream.timeStamp).Milliseconds(), // TODO: get PTS
			})
			break
		case "mdat":
			if fragment, ok := stream.fragments.Load(stream.lastSeqNumber); ok { // handles synchronization internally
				file, _ := memfd.Create()
				file.Write(fragment.(*Fragment).moof)
				file.Write(fullAtom)
				file.SetImmutable()
				// fragment.(*Fragment).data = file // TODO: representation specific fragment file descriptor
				file.Seek(0, io.SeekStart)
				fragment.(*Fragment).byteLength += int(atomSize)
				file.SetSize(int64(fragment.(*Fragment).byteLength))

				clients.Range(func(key, value any) bool {
					client := value.(*Client)
					// client.conn_fd.ReadFrom(file) // TOSTUDY: offset is not implemented in TCPSock_Posix
					// https://cs.opensource.google/go/go/+/refs/tags/go1.19.5:src/net/tcpsock_posix.go;drc=007d8f4db1f890f0d34018bb418bdc90ad4a8c35;l=47

					offset := (int64)(0)
					_, err := syscall.Sendfile(
						int(client.conn_fd.Fd()),
						int(file.Fd()),
						&offset,
						int(fragment.(*Fragment).byteLength))

					if err != nil {
						client.Delete()
					} else {
						tcpinfo, err := tcpinfo.GetsockoptTCPInfo(client.conn.(*net.TCPConn))
						if err != nil {
							fmt.Println("Error getting TCP info:", err)
							// return
						}
						fmt.Printf("%+v\r\r\r\r\r", tcpinfo)
					}

					return true
				})
			}
			break
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

	// #### 1 stream parser ####

	stream := InputStream{}
	go stream.atomParser(namedPipe)

	// #### Start TCP server ####

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

		go handleConnection(&stream, conn)

	}

}

func handleConnection(stream *InputStream, conn net.Conn) {

	fmt.Println("new conn", conn.RemoteAddr())

	conn.Write(stream.moov)

	conn_fd, _ := conn.(*net.TCPConn).File() // unmanaged

	c := &Client{
		conn:    conn,
		conn_fd: conn_fd,
		new:     true,
	}

	clients.Store(conn.RemoteAddr().String(), c)

}
