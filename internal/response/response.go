package response

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/diva-21/http-server/internal/parser"
)

type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

func normalizeHeaders(headers map[string]string) (map[string]string, error) {
	normalized := make(map[string]string)

	for key, value := range headers {
		lowerKey := strings.ToLower(key)

		// validate key val
		if err := parser.ValidateHeader(lowerKey, value); err != nil {
			return nil, err
		}

		// duplicate headers
		if _, exists := normalized[lowerKey]; exists {
			return nil, fmt.Errorf("duplicate response header: %s", key)
		}

		normalized[lowerKey] = value
	}

	return normalized, nil
}

func EnsureHeaders(res *Response) {
	if res.Headers == nil {
		res.Headers = make(map[string]string)
	}
}

func WriteResponse(cn net.Conn, res *Response) error {

	if cn == nil {
		return fmt.Errorf("nil connection")
	}
	if res == nil {
		return fmt.Errorf("nil response")
	}

	if res.StatusCode < 100 || res.StatusCode > 599 {
		return fmt.Errorf("invalid response status code: %d", res.StatusCode)
	}

	EnsureHeaders(res)

	headers, err := normalizeHeaders(res.Headers)

	if err != nil {
		return err
	}

	res.Headers = headers

	res.Headers["content-length"] = strconv.Itoa(len(res.Body))
	res.Headers["connection"] = "close"

	var responseCn bytes.Buffer

	// Status line
	responseCn.WriteString(fmt.Sprintf(
		"HTTP/1.1 %d %s\r\n",
		res.StatusCode,
		StatusText(res.StatusCode),
	))

	for k, v := range res.Headers {
		responseCn.WriteString(k + ":" + " " + v + "\r\n")
	}
	responseCn.WriteString("\r\n")
	responseCn.Write(res.Body)

	_, writeErr := io.Copy(cn, &responseCn)
	if writeErr != nil {
		return fmt.Errorf("writing response: %w", writeErr)
	}

	return nil
}
