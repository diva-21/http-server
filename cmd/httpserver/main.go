package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
)

type ParsedHeaders struct {
	Method           string
	Resource_Path    string
	Protocol_Version string
	Field_Names      map[string][]string
	Body             []byte
}

func parseHeader(reader *bufio.Reader) (ParsedHeaders, error) {
	resParsed := ParsedHeaders{}
	resParsed.Field_Names = make(map[string][]string)

	// 1. Parsing the first request line (method one)
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return resParsed, fmt.Errorf("Error in reading the request line %w", err)
	}

	requestLine = strings.TrimRight(requestLine, "\r\n")
	splitReqLines := strings.Fields(requestLine)

	if len(splitReqLines) != 3 {
		return resParsed, fmt.Errorf("Invalid request %q", requestLine)
	}

	// METHOD /RESOURCE-PATH PROTOCOL-VERSION 	\r\n
	resParsed.Method = splitReqLines[0]
	resParsed.Resource_Path = splitReqLines[1]
	resParsed.Protocol_Version = splitReqLines[2]

	// 2. Parsing the field names / headers ( key : values ) until the blank line
	// example Host: x.com , Content-Length: 11
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Println("err reading request", err)
			return resParsed, fmt.Errorf("Error in reading the header line %w", err)
		}

		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// this only works for GET, not for POST ( it would have request body after end of line )
			fmt.Println("its a \r\n line ")
			break
		}
		headerParts := strings.SplitN(line, ":", 2) // split into exactly 2 ; ex : Content-Length: 11

		if len(headerParts) != 2 {
			return resParsed, fmt.Errorf("Incorrect header: %q", line)
		}
		key := strings.ToLower(strings.TrimSpace(headerParts[0])) // lower for avoid case-insensitive
		value := strings.TrimSpace(headerParts[1])

		if key == "" || value == "" {
			return resParsed, fmt.Errorf("Invalid key-value in header. Key: %q , value : %q", key, value)
		}

		resParsed.Field_Names[key] = append(resParsed.Field_Names[key], value)
	}
	return resParsed, nil
}

func parseBody(reader *bufio.Reader, parsedRes ParsedHeaders) ([]byte, error) {
	// we are checking whether content-length is present or not. if not then no body

	contentLengthStrRes := parsedRes.Field_Names["content-length"]
	if len(contentLengthStrRes) == 0 {
		// no content length header is present
		fmt.Println("No content length header is present")
		return nil, nil
	}
	contentLengthStr := contentLengthStrRes[0] // content-length will be once in header
	contentLength, err := strconv.Atoi(contentLengthStr)
	if err != nil {
		return nil, fmt.Errorf("Invalid content-length")
	}

	// make []byte of contentLength
	bodyBuffer := make([]byte, contentLength)

	n, readErr := io.ReadFull(reader, bodyBuffer)

	if readErr != nil {

		if readErr == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("Interruption ; Read %d/%d bytes : %w", n, contentLength, readErr)
		}
		return nil, readErr
	}

	return bodyBuffer, nil

}
func handleConnection(cn net.Conn, wg *sync.WaitGroup) {
	// cn management
	defer cn.Close()
	defer wg.Done()

	fmt.Println("Connection ", cn)

	// request reading and parsing
	// Drain the HTTP request first. Closing a TCP socket while unread
	// request bytes remain in the receive buffer makes Windows send RST
	reader := bufio.NewReader(cn)

	parsedHeadersResult, parseErr := parseHeader(reader)
	if parseErr != nil {
		log.Println("Error in parsing the header lines", parseErr)
		return
	}
	bodyBuf, parseBodyErr := parseBody(reader, parsedHeadersResult)

	if parseBodyErr != nil {
		log.Println("Error in parsing the body", parseBodyErr)
		return
	}

	fmt.Println("We parsed and got this", parsedHeadersResult)
	fmt.Println("And body is %q",string(bodyBuf))

	// response formatting
	resBody := "Hello, World!\n"
	response := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain\r\n" +
		fmt.Sprintf("Content-Length: %d\r\n", len(resBody)) +
		"\r\n" +
		resBody

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

func getLinesChannel(f io.ReadCloser) <-chan string {
	strChan := make(chan string)
	buffer := make([]byte, 8)
	str := ""

	go func() {

		defer f.Close()
		defer close(strChan)

		for {
			n, readErr := f.Read(buffer)
			if n > 0 {
				data := buffer[:n]

				if i := bytes.IndexByte(data, '\n'); i != -1 {
					// means i got a end of line, so i need to add it to my main str which
					// i used for concatinating the buf contents till now & print the str now. (post adding the current one)
					// then i will move the data to the next char of \n.
					//
					str += string(data[:i])
					data = data[i+1:]
					// fmt.Printf("read: %s\n", str)
					strChan <- str
					str = ""
				}
				// if its not an end of line, iwill simply add the strs
				str += string(data)
			}

			if readErr == io.EOF {
				if str != "" {
					// fmt.Printf("read: %s\n", str)
					strChan <- str
				}
				break
			}
			if readErr != nil {
				log.Fatal("Error in reading", readErr)
				break
			}
		}
	}()

	return strChan
}

func fileOpenReadTest() {
	file, err := os.Open("messages.txt")
	if err != nil {
		log.Fatal("Error in opening the file")
		return
	}

	// defer file.Close()

	recChan := getLinesChannel(file)

	for res := range recChan {
		fmt.Printf("read: %s\n", res)
	}
}
