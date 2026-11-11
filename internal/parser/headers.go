package parser

import (
	"fmt"
	"strings"
)

func isTokenChar(c byte) bool {
	if c >= 'a' && c <= 'z' ||
		c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' {
		return true
	}

	return strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c))
}

func ValidateHeader(key, value string) error {
	if key == "" {
		return fmt.Errorf("empty header name")
	}

	for i := 0; i < len(key); i++ {
		if !isTokenChar(key[i]) {
			return fmt.Errorf("invalid header name: %q", key)
		}
	}

	if strings.ContainsAny(key, "\r\n:") {
		return fmt.Errorf("invalid header name: %q", key)
	}

	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("invalid header value for %q", key)
	}

	return nil
}
