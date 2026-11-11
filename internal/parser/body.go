package parser

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/diva-21/http-server/internal/httperror"
)

// just consume from the reader, dont store for now.
func consumeTrailers(reader *bufio.Reader) error {

	totalSize := 0
	count := 0

	for {
		line, err := ReadLineWithLimit(reader, maxTrailerSize)
		if err != nil {
			return &httperror.HTTPError{
				StatusCode: 400,
				Message:    "invalid chunk trailers",
				Err:        err,
			}
		}

		totalSize += len(line) + 2 // include CRLF

		if totalSize > maxTrailerSize {
			return &httperror.HTTPError{
				StatusCode: 431,
				Message:    "trailer section too large",
			}
		}

		// Empty line means trailer section is finished.
		if line == "" {
			return nil
		}

		count++

		if count > maxTrailerCount {
			return &httperror.HTTPError{
				StatusCode: 431,
				Message:    "too many trailer fields",
			}
		}

		key, value, found := strings.Cut(line, ":")

		if !found || key == "" {
			return &httperror.HTTPError{
				StatusCode: 400,
				Message:    "invalid trailer field",
			}
		}

		for i := 0; i < len(key); i++ {
			if !isTokenChar(key[i]) {
				return &httperror.HTTPError{
					StatusCode: 400,
					Message:    "invalid trailer field name",
				}
			}
		}

		value = strings.Trim(value, " \t")

		for i := 0; i < len(value); i++ {
			if value[i] < 32 && value[i] != '\t' ||
				value[i] == 127 {
				return &httperror.HTTPError{
					StatusCode: 400,
					Message:    "invalid trailer field value",
				}
			}
		}

		// Do not accept framing or routing fields as trailers.
		switch strings.ToLower(key) {
		case "content-length", "transfer-encoding", "host", "trailer":
			return &httperror.HTTPError{
				StatusCode: 400,
				Message:    "forbidden trailer field",
			}
		}
	}
}
func parseChunkedBody(reader *bufio.Reader, maxBodySize int) ([]byte, error) {
	// extract chunk size
	// convert chunk size (hexa) to int
	// check chunk size == 0; if yes, then check if they're any trailers present
	// check max size limit
	// read chunk size no. of bytes to body
	// CRITICAL: at last , post reading the above bytes, check it ends with CRLF

	var body []byte

	for {

		line, err := ReadLineWithLimit(reader, maxRequestLine)
		if err != nil {
			return nil, &httperror.HTTPError{
				StatusCode: 400,
				Message:    "invalid chunk line size",
				Err:        err,
			}
		}

		// extract chunk size from "10;ext1;ext2=hello"
		// chunkSize;extraData;....
		sizeText, _, _ := strings.Cut(line, ";")
		if sizeText == "" {
			return nil, &httperror.HTTPError{StatusCode: 400, Message: "empty chunk size"}
		}

		// chunk size ( hexa to 64 int )
		chunkSize, err := strconv.ParseUint(sizeText, 16, 64)
		if err != nil {
			return nil, &httperror.HTTPError{StatusCode: 400, Message: "invalid chunk size", Err: err}
		}

		// if chunkSize == 0 , then check for trailers; Trailers = late-calculated metadata
		if chunkSize == 0 {
			// check for trailers
			if err := consumeTrailers(reader); err != nil {
				return nil, err
			}
			return body, nil
		}

		// check max size limit
		if chunkSize > uint64(maxBodySize-len(body)) {
			return nil, &httperror.HTTPError{StatusCode: 413, Message: "request body too large"}
		}

		//read chunkSize no. of bytes to body
		chunk := make([]byte, int(chunkSize))

		if _, err := io.ReadFull(reader, chunk); err != nil {
			return nil, &httperror.HTTPError{StatusCode: 400, Message: "incomplete chunk data", Err: err}
		}

		body = append(body, chunk...)

		// chunk data must be followed by CRLF
		var ending [2]byte
		if _, err = io.ReadFull(reader, ending[:]); err != nil || ending != [2]byte{'\r', '\n'} {
			return nil, &httperror.HTTPError{StatusCode: 400, Message: "invalid chunk ending", Err: err}
		}

	}
}
func ParseBody(reader *bufio.Reader, req *Request) ([]byte, error) {

	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	transferEncodings := req.Headers["transfer-encoding"]
	contentLengths := req.Headers["content-length"]

	if len(transferEncodings) > 0 && len(contentLengths) > 0 {
		// both cant exist at once
		return nil, &httperror.HTTPError{
			StatusCode: 400,
			Message:    "Transfer-Encoding & Content-Length both cant be present",
		}
	}

	// handle transfer encoding
	if len(transferEncodings) > 0 {
		// rm dups
		if len(transferEncodings) != 1 {
			return nil, &httperror.HTTPError{
				StatusCode: 400,
				Message:    "multiple Transfer-Encoding headers",
			}
		}

		encoded := strings.TrimSpace(transferEncodings[0])
		// Transfer-Encoding: chunked

		if !strings.EqualFold(encoded, "chunked") {
			// only chunked is accepted
			return nil, &httperror.HTTPError{
				StatusCode: 501,
				Message:    "Transfer-Encoding is not supported",
			}
		}

		// retrieving body from the stream 5\r\nHello\r\n0
		body, err := parseChunkedBody(reader, maxBodySize)
		if err != nil {
			return nil, err
		}

		req.Body = body
		return body, nil

	}

	// handle content length
	if len(contentLengths) == 0 {
		req.Body = nil
		return nil, nil
	}

	if len(contentLengths) != 1 {
		return nil, &httperror.HTTPError{
			StatusCode: 400,
			Message:    "multiple Content-Length headers",
		}
	}

	contentLength, err := strconv.Atoi(contentLengths[0])
	if err != nil || contentLength < 0 {
		return nil, &httperror.HTTPError{
			StatusCode: 400,
			Message:    "invalid Content-Length",
		}
	}

	if contentLength > maxBodySize {
		return nil, &httperror.HTTPError{
			StatusCode: 413,
			Message:    "Request body too large",
		}
	}

	bodyBuffer := make([]byte, contentLength)

	// CHANGE 1: Capture the number of bytes read ('n') and use 'readErr' explicitly
	n, readErr := io.ReadFull(reader, bodyBuffer)
	if readErr != nil {
		// CHANGE 2: Your specific check for unexpected EOF
		if readErr == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("Interruption ; Read %d/%d bytes : %w", n, contentLength, readErr)
		}

		// Fallback for any other read errors
		return nil, &httperror.HTTPError{
			StatusCode: 400,
			Message:    "Incomplete request body",
			Err:        readErr,
		}
	}

	req.Body = bodyBuffer

	return bodyBuffer, nil
}
