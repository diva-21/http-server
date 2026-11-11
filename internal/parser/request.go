package parser

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/diva-21/http-server/internal/httperror"
)

type Request struct {
	Method          string
	ResourcePath    string
	ProtocolVersion string
	Headers         map[string][]string
	Body            []byte
	QueryParams     url.Values
}

const (
	maxRequestLine = 8 * 1024
	maxHeaderSize  = 32 * 1024
	maxHeaderCount = 100
	maxBodySize    = 1 << 20 // 1 MiB

	maxTrailerSize  = 32 * 1024
	maxTrailerCount = 100
)

func ReadLineWithLimit(reader *bufio.Reader, maxLimit int) (string, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')

		if len(part)+len(line) > maxLimit {
			return "", fmt.Errorf("line exceeded maximum size")
		}
		line = append(line, part...)

		if err == nil {
			// got one full line , break
			break
		}
		if err != bufio.ErrBufferFull {
			// other error in reading
			return "", err
		}
		// err == ErrBufferFull means, continue & go to next iteration and stich until we get "\n"
	}

	// when we exit the loop, we got one full line, now check conditions on that full line
	// 1. ends with '\n'
	// 2. last one before char is '\r'
	// 3. at least size should be 2 ; (\r\n) is min req in a line

	if len(line) < 2 || line[len(line)-1] != '\n' || line[len(line)-2] != '\r' {
		return "", fmt.Errorf("invalid line ending")
	}

	return string(line[:len(line)-2]), nil // strip CRLF (\r\n)

}
func ParseHeader(reader *bufio.Reader) (Request, error) {
	parsedReq := Request{}
	parsedReq.Headers = make(map[string][]string)

	// 1. Parsing the first request line (method one)
	// requestLine, err := reader.ReadString('\n') // old
	requestLine, err := ReadLineWithLimit(reader, maxRequestLine)

	if err != nil {
		return parsedReq, fmt.Errorf("Error in reading the request line %w", err)
	}

	requestLine = strings.TrimRight(requestLine, "\r\n")
	splitReqLines := strings.Fields(requestLine)

	if len(splitReqLines) != 3 {
		return parsedReq, fmt.Errorf("Invalid request %q", requestLine)
	}

	// METHOD /RESOURCE-PATH PROTOCOL-VERSION 	\r\n
	parsedReq.Method = splitReqLines[0]
	parsedReq.ProtocolVersion = splitReqLines[2]

	parsedReq.ResourcePath = splitReqLines[1]

	target, err := url.ParseRequestURI(splitReqLines[1])
	if err != nil ||
		target.IsAbs() ||
		target.Host != "" ||
		target.Fragment != "" ||
		!strings.HasPrefix(target.Path, "/") {

		return parsedReq, &httperror.HTTPError{
			StatusCode: 400,
			Message:    "Invalid request target",
		}
	}

	// store only the resource path
	parsedReq.ResourcePath = target.Path // contains /users

	// extract query parameters
	parsedReq.QueryParams, err = url.ParseQuery(target.RawQuery) // rawQuery contains id=1&name=Sai
	if err != nil {
		return parsedReq, &httperror.HTTPError{
			StatusCode: 400,
			Message:    "Invalid query parameters",
			Err:        err,
		}
	}

	// validate for Method
	for i := 0; i < len(parsedReq.Method); i++ {
		if !isTokenChar(parsedReq.Method[i]) {
			return parsedReq, fmt.Errorf("invalid HTTP method")
		}
	}

	// validate for http version
	if parsedReq.ProtocolVersion != "HTTP/1.1" {
		return parsedReq, &httperror.HTTPError{
			StatusCode: 505,
			Message:    "HTTP Version Not Supported",
		}
	}

	// 2. Parsing the field names / headers ( key : values ) until the blank line
	// example Host: x.com , Content-Length: 11

	headerCount := 0
	totalHeaderSize := 0
	for {
		// line, err := reader.ReadString('\n') // old
		line, err := ReadLineWithLimit(reader, maxHeaderSize)
		if err != nil {
			log.Println("err reading request", err)
			return parsedReq, fmt.Errorf("Error in reading the header line %w", err)
		}

		totalHeaderSize += len(line) + 2 // include CRLF

		if totalHeaderSize > maxHeaderSize {
			return parsedReq, &httperror.HTTPError{
				StatusCode: 431,
				Message:    "Request Header Fields Too Large",
			}
		}

		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			fmt.Println("its a \r\n line ")
			break
		}

		headerCount++

		if headerCount > maxHeaderCount {
			return parsedReq, &httperror.HTTPError{
				StatusCode: 431,
				Message:    "too many request headers",
			}
		}
		headerParts := strings.SplitN(line, ":", 2) // split into exactly 2 ; ex : Content-Length: 11

		if len(headerParts) != 2 {
			return parsedReq, fmt.Errorf("incorrect header: %q", line)
		}
		key := strings.ToLower(strings.TrimSpace(headerParts[0])) // lower for avoid case-insensitive
		value := strings.TrimSpace(headerParts[1])

		if key == "" {
			return parsedReq, fmt.Errorf("invalid header format")
		}

		// Validate header name
		for i := 0; i < len(key); i++ {
			if !isTokenChar(key[i]) {
				return parsedReq, fmt.Errorf("invalid header name: %q", key)
			}
		}

		parsedReq.Headers[key] = append(parsedReq.Headers[key], value)
	}
	return parsedReq, nil
}
