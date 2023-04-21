package tritonhttp

import (
	"bufio"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	RESPONSE_PROTO     = "HTTP/1.1"
	STATUS_OK          = 200
	STATUS_BAD_REQUEST = 400
	STATUS_NOT_FOUND   = 404
)

type Server struct {
	// Addr specifies the TCP address for the server to listen on,
	// in the form "host:port". It shall be passed to net.Listen()
	// during ListenAndServe().
	Addr string // e.g. ":0"

	// VirtualHosts contains a mapping from host name to the docRoot path
	// (i.e. the path to the directory to serve static files from) for
	// all virtual hosts that this server supports
	VirtualHosts map[string]string
}

var statusText = map[int]string{
	STATUS_OK:          "OK",
	STATUS_BAD_REQUEST: "Bad Request",
	STATUS_NOT_FOUND:   "Not Found",
}

// func checkErr(err error) {
// 	if err != nil {
// 		log.Panicln(err)
// 	}
// }

// ListenAndServe listens on the TCP network address s.Addr and then
// handles requests on incoming connections.
func (s *Server) ListenAndServe() error {

	// Hint: create your listen socket and spawn off goroutines per incoming client
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept Connection Error: ", err)
			continue
		}
		log.Printf("conn address: %s\n", conn.RemoteAddr().String())
		go s.HandleConnection(conn)
	}

}

func (s *Server) HandleConnection(conn net.Conn) {
	bufferR := bufio.NewReader(conn)

	for {
		// set timeout for 5 seconds
		err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err == io.EOF {
			// log.Println("EOF")
			conn.Close()
			return
		}

		// read request
		request, err := s.ReadRequest(bufferR)

		// handle errors
		// 1. client has closed the connection
		if err == io.EOF {
			log.Println("client has closed the connection: ", conn.RemoteAddr().String())
			//request.Close = true
			conn.Close()
			return
		}
		// 2. server is timeout
		if err, ok := err.(net.Error); ok && err.Timeout() {
			log.Printf("Connection to %q timed out.\n", conn.RemoteAddr().String())
			//request.Close = true
			conn.Close()
			return
		}
		// 3. invalid/malformed request
		if err != nil {
			log.Println(err)

			response := &Response{}
			response.Headers = map[string]string{}
			response.HandleBadRequest()
			err = response.WriteToResponse(conn)
			if err != nil {
				log.Println(err)
			}
			//request.Close = true
			conn.Close()
			return
		}

		// handle valid request
		// response := &Response{}
		response := s.HandleGoodRequest(request)
		err = response.WriteToResponse(conn)
		if err != nil {
			log.Println(err)
		}

	}

}

func (s *Server) HandleGoodRequest(request *Request) *Response {
	response := &Response{}
	response.Headers = map[string]string{}
	host := request.Host
	_, ok := s.VirtualHosts[host]
	if !ok {
		log.Printf("the server doesn't support host %q: \n", host)
		response.FilePath = request.URL
		response.Request = request
		response.HandleNotFound()
		return response
	}

	//1. if path end with '/', map '/' to /index.html
	// i.e. aaa/bbb => aaa/bbb/index.html
	if request.URL[len(request.URL)-1] == '/' {
		request.URL = path.Clean(request.URL + "index.html")
	}

	// check if URL is valid, if not, return 404
	// 2. If the URL is something other than '/', then translate the URL to a filename.
	// Run the 'stat' command on the filename and check if it is a file or a directory
	absoluteURL := s.VirtualHosts[host] + path.Clean(request.URL)
	fileInfo, err := os.Stat(absoluteURL)

	if os.IsNotExist(err) {
		log.Println("File/Dir doesn't exit: " + absoluteURL)
		response.FilePath = absoluteURL
		response.Request = request
		response.HandleNotFound()
		return response
	}

	// 2a. If it is a directory, add '/index.html' to the end and serve that out
	// (and if that doesn't exist, return a 404)
	if fileInfo.IsDir() {
		request.URL = path.Clean(request.URL + "/index.html")
		absoluteURL = s.VirtualHosts[host] + path.Clean(request.URL)
		fileInfo, err = os.Stat(absoluteURL)
		if os.IsNotExist(err) {
			log.Println("File/Dir doesn't exit: " + absoluteURL)
			response.FilePath = absoluteURL
			response.Request = request
			response.HandleNotFound()
			return response
		}
	}

	// 2b. If it is a file, use the extension to find the mime-type, and just serve it out
	// if everything is good
	response.FilePath = absoluteURL
	response.Request = request
	response.Headers["Last-Modified"] = FormatTime(fileInfo.ModTime())
	response.Headers["Content-Length"] = strconv.Itoa(int(fileInfo.Size()))
	fileExt := filepath.Ext(absoluteURL)
	//log.Println("absolute URL: ", absoluteURL)
	response.Headers["Content-Type"] = strings.SplitN(MIMETypeByExtension(fileExt), ";", 2)[0]
	response.HandleOk()

	return response

}

func (response *Response) HandleBadRequest() {
	response.Proto = RESPONSE_PROTO
	response.StatusCode = STATUS_BAD_REQUEST
	response.StatusText = statusText[response.StatusCode]
	response.Request = nil
	response.FilePath = ""
	response.Headers["Connection"] = "close"
	response.Headers["Date"] = FormatTime(time.Now())
}

func (response *Response) HandleNotFound() {
	response.Proto = RESPONSE_PROTO
	response.StatusCode = STATUS_NOT_FOUND
	response.StatusText = statusText[response.StatusCode]
	if response.Request.Close {
		response.Headers["Connection"] = "close"
	}
	response.Headers["Date"] = FormatTime(time.Now())
}

func (response *Response) HandleOk() {
	response.Proto = RESPONSE_PROTO
	response.StatusCode = STATUS_OK
	response.StatusText = statusText[response.StatusCode]
	if response.Request.Close {
		response.Headers["Connection"] = "close"
	}
	response.Headers["Date"] = FormatTime(time.Now())
}

// func (s *Server) ValidateDocRoots() error {
// 	fmt.Println("s.addr: ", s.Addr)
// 	host := strings.SplitN(s.Addr, ":", 2)[0]
// 	docRoot, ok := s.VirtualHosts[host]
// 	if !ok {
// 		return fmt.Errorf("the host %q is not supported by the server", host)
// 	}

// 	fileInfo, err := os.Stat(docRoot)

// 	if os.IsNotExist(err) {
// 		return err
// 	}

// 	if !fileInfo.IsDir() {
// 		return fmt.Errorf("doc root %q is not a directory", docRoot)
// 	}

// 	return nil

// }
