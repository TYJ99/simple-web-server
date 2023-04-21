package tritonhttp

import (
	"bufio"
	"fmt"
	"strings"
)

type Request struct {
	Method string // e.g. "GET"
	URL    string // e.g. "/path/to/a/file"
	Proto  string // e.g. "HTTP/1.1"

	// Headers stores the key-value HTTP headers
	Headers map[string]string

	Host  string // determine from the "Host" header
	Close bool   // determine from the "Connection" header
}

func (s *Server) ReadRequest(bufferR *bufio.Reader) (*Request, error) {
	request := &Request{}
	request.Headers = map[string]string{}

	// read initial request line
	line, err := ReadLine(bufferR)
	if err != nil {
		return nil, err
	}

	initialRequest, err := ParseInitialRequestLine(line)
	if err != nil {
		return nil, err
	}
	// make sure method field == "GET"
	if !ValidateMethodField(initialRequest[0]) {
		return nil, fmt.Errorf("invalid Method Found: %s", initialRequest[0])
	}

	if !ValidateURL(initialRequest[1]) {
		return nil, fmt.Errorf("invalid URL: %s", initialRequest[1])
	}

	if !ValidateProto(initialRequest[2]) {
		return nil, fmt.Errorf("invalid proto: %s", initialRequest[2])
	}

	request.Method = initialRequest[0]
	request.URL = initialRequest[1]
	request.Proto = RESPONSE_PROTO

	// read more header lines(key-value pairs)
	for {
		line, err := ReadLine(bufferR)
		if err != nil {
			return nil, err
		}

		// end of request
		if line == "" {
			break
		}

		header, err := ParseHeaderLine(line)
		if err != nil {
			return nil, err
		}

		key := header[0]
		value := header[1]
		if key == "Host" {
			request.Host = value
		}
		if key == "Connection" {
			if value == "close" {
				request.Close = true
			}
		}
		request.Headers[key] = value
	}
	return request, nil

}

func ParseHeaderLine(line string) ([]string, error) {
	splitLine := strings.SplitN(line, ":", 2)
	if len(splitLine) != 2 {
		return nil, fmt.Errorf("parse header Line failed(no colon): %v", splitLine)
	}

	// <key> is composed of one or more alphanumeric or the hyphen "-" character
	// (i.e. <key> cannot be empty).
	// It is case-insensitive.

	if splitLine[0] == "" {
		return nil, fmt.Errorf("<key> cannot be empty")
	}
	splitLine[0] = CanonicalHeaderKey(strings.TrimSpace(splitLine[0]))

	// <value> can be any string not starting with space, and not containing CRLF. It is
	// case-sensitive. As a special case <value> can be an empty string.
	splitLine[1] = strings.TrimSpace(splitLine[1])

	return splitLine, nil
}

func ValidateMethodField(field string) bool {
	return field == "GET"
}

func ValidateURL(URL string) bool {
	return URL[:1] == "/"
}

func ValidateProto(proto string) bool {
	return proto == RESPONSE_PROTO
}

func ParseInitialRequestLine(line string) ([]string, error) {
	splitLine := strings.SplitN(line, " ", 3)
	if len(splitLine) != 3 {
		return nil, fmt.Errorf("parse Request Line failed: %v", splitLine)
	}
	splitLine[0] = strings.TrimSpace(splitLine[0])
	splitLine[1] = strings.TrimSpace(splitLine[1])
	splitLine[2] = strings.TrimSpace(splitLine[2])
	return splitLine, nil
}

func ReadLine(bufferR *bufio.Reader) (string, error) {
	var line string
	for {
		str, err := bufferR.ReadString('\n')
		line += str
		if err != nil {
			return line, err
		}

		if strings.HasSuffix(line, "\r\n") {
			line = line[:len(line)-2]
			return line, nil
		}

	}
}
