package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client is a thin IPC client for the daemon socket. It speaks the same
// newline-delimited JSON protocol as the server.
type Client struct {
	conn net.Conn
	r    *bufio.Reader
}

// Dial connects to the daemon socket.
func Dial(sockPath string) (*Client, error) {
	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, r: bufio.NewReaderSize(conn, 64*1024)}, nil
}

// Close closes the connection.
func (c *Client) Close() error { return c.conn.Close() }

// Do sends a request and reads one response.
func (c *Client) Do(req Request) (*Response, error) {
	req.Version = ProtocolVersion
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if _, err := c.conn.Write(data); err != nil {
		return nil, err
	}
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("daemon: %s", resp.Error)
	}
	return &resp, nil
}

// Hello performs the version/capabilities handshake.
func (c *Client) Hello() (*HelloResp, error) {
	resp, err := c.Do(Request{Type: ReqHello})
	if err != nil {
		return nil, err
	}
	return resp.Hello, nil
}
