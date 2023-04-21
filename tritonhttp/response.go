package tritonhttp

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
)

type Response struct {
	Proto      string // e.g. "HTTP/1.1"
	StatusCode int    // e.g. 200
	StatusText string // e.g. "OK"

	// Headers stores all headers to write to the response.
	Headers map[string]string

	// Request is the valid request that leads to this response.
	// It could be nil for responses not resulting from a valid request.
	// Hint: you might need this to handle the "Connection: Close" requirement
	Request *Request

	// FilePath is the local path to the file to serve.
	// It could be "", which means there is no file to serve.
	FilePath string
}

func (response *Response) WriteToResponse(writer io.Writer) error {
	bufferW := bufio.NewWriter(writer)
	// write initial Response Line
	initialResponseLine := fmt.Sprintf("%v %v %v\r\n", response.Proto, response.StatusCode, response.StatusText)
	_, err := bufferW.WriteString(initialResponseLine)
	if err != nil {
		return err
	}
	err = bufferW.Flush()
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(response.Headers))
	for key := range response.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// write Response headers if there is any
	for _, key := range keys {
		headerLine := fmt.Sprintf("%v: %v\r\n", key, response.Headers[key])
		//log.Println("header line: ", headerLine)
		_, err := bufferW.WriteString(headerLine)
		if err != nil {
			return err
		}
		err = bufferW.Flush()
		if err != nil {
			return err
		}
	}

	_, err = bufferW.WriteString("\r\n")
	if err != nil {
		return err
	}
	err = bufferW.Flush()
	if err != nil {
		return err
	}

	// write message body if status OK (200)
	if response.StatusCode == 200 {
		file, err := os.Open(response.FilePath)
		if err != nil {
			return err
		}
		defer file.Close()

		buffer, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}
		_, err = bufferW.Write(buffer)
		if err != nil {
			return err
		}
		err = bufferW.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}
