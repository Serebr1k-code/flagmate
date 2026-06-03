package eve

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Event struct {
	Timestamp     string                 `json:"timestamp"`
	EventType     string                 `json:"event_type"`
	SrcIP         string                 `json:"src_ip"`
	SrcPort       int                    `json:"src_port"`
	DstIP         string                 `json:"dst_ip"`
	DstPort       int                    `json:"dst_port"`
	Proto         string                 `json:"proto"`
	FlowID        uint64                 `json:"flow_id"`
	HTTP          *HTTPEvent             `json:"http,omitempty"`
	Alert         *AlertEvent            `json:"alert,omitempty"`
	Flow          *FlowEvent             `json:"flow,omitempty"`
	TLS           *TLSEvent              `json:"tls,omitempty"`
	DNS           map[string]interface{} `json:"dns,omitempty"`
	Stats         map[string]interface{} `json:"stats,omitempty"`
	Extra         map[string]interface{} `json:"-"`
}

type HTTPEvent struct {
	HTTPRequest  *HTTPRequest  `json:"request,omitempty"`
	HTTPResponse *HTTPResponse `json:"response,omitempty"`
}

type HTTPRequest struct {
	Method        string      `json:"request_method"`
	URI           string      `json:"request_uri"`
	Protocol      string      `json:"request_protocol"`
	Status        int         `json:"status"`
	Headers       interface{} `json:"request_headers"`
	Body          interface{} `json:"request_body"`
	BodyPrintable interface{} `json:"request_body_printable"`
}

type HTTPResponse struct {
	Status        int         `json:"status"`
	Protocol      string      `json:"response_protocol"`
	Headers       interface{} `json:"response_headers"`
	Body          interface{} `json:"response_body"`
	BodyPrintable interface{} `json:"response_body_printable"`
}

type AlertEvent struct {
	Action      string `json:"action"`
	Signature   string `json:"signature"`
	Category    string `json:"category"`
	Severity    int    `json:"severity"`
	SignatureID int    `json:"signature_id"`
	Rev         int    `json:"rev"`
}

type FlowEvent struct {
	PktsToserver  int    `json:"pkts_toserver"`
	PktsToclient  int    `json:"pkts_toclient"`
	BytesToserver int64  `json:"bytes_toserver"`
	BytesToclient int64  `json:"bytes_toclient"`
	Start         string `json:"start"`
	End           string `json:"end,omitempty"`
	State         string `json:"state"`
	Reason        string `json:"reason,omitempty"`
}

type TLSEvent struct {
	Version     string `json:"version"`
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	SNI         string `json:"sni"`
	NotBefore   string `json:"notbefore"`
	NotAfter    string `json:"notafter"`
	Fingerprint string `json:"fingerprint"`
}

type Reader struct {
	source      string
	isSocket    bool
	mu          sync.Mutex
	subscribers []chan Event
	done        chan struct{}
}

func NewReader(source string) *Reader {
	isSocket := strings.HasSuffix(source, ".sock")
	return &Reader{
		source:   source,
		isSocket: isSocket,
		done:     make(chan struct{}),
	}
}

func (r *Reader) Subscribe() chan Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan Event, 1000)
	r.subscribers = append(r.subscribers, ch)
	return ch
}

func (r *Reader) Start() error {
	if r.isSocket {
		return r.startSocket()
	}
	return r.startFile()
}

func (r *Reader) startSocket() error {
	for {
		select {
		case <-r.done:
			return nil
		default:
		}

		conn, err := net.Dial("unix", r.source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to EVE socket %s: %v, retrying in 2s...\n", r.source, err)
			time.Sleep(2 * time.Second)
			continue
		}

		fmt.Println("Connected to EVE socket")
		r.readEventsFromConn(conn)
		conn.Close()
		time.Sleep(1 * time.Second)
	}
}

func (r *Reader) startFile() error {
	// Open file once and keep reading from end (tail -f behavior)
	for {
		select {
		case <-r.done:
			return nil
		default:
		}

		file, err := os.Open(r.source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open EVE file %s: %v, retrying in 2s...\n", r.source, err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Seek to end to only read new events
		file.Seek(0, 2)
		fmt.Printf("Tailing EVE file: %s\n", r.source)
		r.tailFile(file)
		file.Close()
		time.Sleep(1 * time.Second)
	}
}

func (r *Reader) tailFile(file *os.File) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for {
		select {
		case <-r.done:
			return
		default:
		}

		if scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event Event
			if err := json.Unmarshal(line, &event); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to unmarshal EVE event: %v\n", err)
				continue
			}

			r.broadcast(event)
		} else {
			// No new data, wait a bit
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (r *Reader) readEventsFromConn(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to unmarshal EVE event: %v\n", err)
			continue
		}

		r.broadcast(event)
	}
}

func (r *Reader) broadcast(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ch := range r.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (r *Reader) Stop() {
	close(r.done)
}
