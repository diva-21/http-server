package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

func handleConnection(cn net.Conn, wg *sync.WaitGroup) {
	// cn management
	defer cn.Close()
	defer wg.Done()

	fmt.Println("Connection ", cn)

	// request reading and parsing
	// Drain the HTTP request first. Closing a TCP socket while unread
	// request bytes remain in the receive buffer makes Windows send RST
	reader := bufio.NewReader(cn)
	for {
		line, err := reader.ReadString('\n')
		fmt.Println("Line is ", line)
		if err != nil {
			log.Println("err reading request", err)
			return
		}
		if line == "\r\n" || line == "\n" {
			// this only works for GET, not for POST ( it would have request body after end of line )
			break
		}
	}

	// response formatting
	body := "Hello, World!\n"
	response := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain\r\n" +
		fmt.Sprintf("Content-Length: %d\r\n", len(body)) +
		"\r\n" +
		body

	_, err := cn.Write([]byte(response))
	if err != nil {
		log.Println("err in writing the response", err)
		return
	}
	fmt.Println("Response written successfully")
}

func startServer() {
	// my tcp needs to the following:
	//
	// Reading the HTTP request headers & parsing
	// Body reading & parsing
	// Response formatting
	// Connection management

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("error ", err)
	}
	defer listener.Close()

	wg := sync.WaitGroup{}

	fmt.Println("Running on 8080 port")

	for {
		cn, err := listener.Accept()
		if err != nil {
			log.Println("Error in accept", err)
			continue
		}
		wg.Add(1)
		go handleConnection(cn, &wg)
	}
}
func main() {
	// fileOpenReadTest()
	startServer()
}

func fileOpenReadTest() {
	file, err := os.Open("messages.txt")
	if err != nil {
		log.Fatal("Error in opening the file")
		return
	}

	defer file.Close()

	buffer := make([]byte, 8)
	str := ""
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			data := buffer[:n]

			if i := bytes.IndexByte(data, '\n'); i != -1 {
				// means i got a end of line, so i need to add it to my main str which
				// i used for concatinating the buf contents till now & print the str now. (post adding the current one)
				// then i will move the data to the next char of \n.
				//
				str += string(data[:i])
				data = data[i+1:]
				fmt.Printf("read: %s\n", str)
				str = ""
			}
			// if its not an end of line, iwill simply add the strs
			str += string(data)
		}

		if readErr == io.EOF {
			if str != "" {
				fmt.Printf("read: %s\n", str)
			}
			break
		}
		if readErr != nil {
			log.Fatal("Error in reading", readErr)
			return
		}
	}
}
