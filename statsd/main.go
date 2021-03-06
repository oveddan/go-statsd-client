package statsd

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"
)

type Statter interface {
	Inc(stat string, value int64, rate float32) error
	Dec(stat string, value int64, rate float32) error
	Gauge(stat string, value int64, rate float32) error
	GaugeDelta(stat string, value int64, rate float32) error
	Timing(stat string, delta int64, rate float32) error
	TimingDuration(stat string, delta time.Duration, rate float32) error
	Raw(stat string, value string, rate float32) error
	SetPrefix(prefix string)
	Close() error
}

type Sender interface {
	Send(data []byte) (int, error)
	Close() error
}

type Client struct {
	// prefix for statsd name
	prefix string
	// packet sender
	sender Sender
}

// Close closes the connection and cleans up.
func (s *Client) Close() error {
	if s == nil {
		return nil
	}
	err := s.sender.Close()
	return err
}

// Increments a statsd count type.
// stat is a string name for the metric.
// value is the integer value
// rate is the sample rate (0.0 to 1.0)
func (s *Client) Inc(stat string, value int64, rate float32) error {
	dap := fmt.Sprintf("%d|c", value)
	return s.Raw(stat, dap, rate)
}

// Decrements a statsd count type.
// stat is a string name for the metric.
// value is the integer value.
// rate is the sample rate (0.0 to 1.0).
func (s *Client) Dec(stat string, value int64, rate float32) error {
	return s.Inc(stat, -value, rate)
}

// Submits/Updates a statsd gauge type.
// stat is a string name for the metric.
// value is the integer value.
// rate is the sample rate (0.0 to 1.0).
func (s *Client) Gauge(stat string, value int64, rate float32) error {
	dap := fmt.Sprintf("%d|g", value)
	return s.Raw(stat, dap, rate)
}

// Submits a delta to a statsd gauge.
// stat is the string name for the metric.
// value is the (positive or negative) change.
// rate is the sample rate (0.0 to 1.0).
func (s *Client) GaugeDelta(stat string, value int64, rate float32) error {
	dap := fmt.Sprintf("%+d|g", value)
	return s.Raw(stat, dap, rate)
}

// Submits a statsd timing type.
// stat is a string name for the metric.
// delta is the time duration value in milliseconds
// rate is the sample rate (0.0 to 1.0).
func (s *Client) Timing(stat string, delta int64, rate float32) error {
	dap := fmt.Sprintf("%d|ms", delta)
	return s.Raw(stat, dap, rate)
}

// Submits a statsd timing type.
// stat is a string name for the metric.
// delta is the timing value as time.Duration
// rate is the sample rate (0.0 to 1.0).
func (s *Client) TimingDuration(stat string, delta time.Duration, rate float32) error {

	ms := float64(delta) / float64(time.Millisecond)

	dap := fmt.Sprintf("%.02f|ms", ms)
	return s.Raw(stat, dap, rate)
}

// Raw formats the statsd event data, handles sampling, prepares it,
// and sends it to the server.
// stat is the string name for the metric.
// value is a preformatted "raw" value string.
// rate is the sample rate (0.0 to 1.0).
func (s *Client) Raw(stat string, value string, rate float32) error {
	if s == nil {
		return nil
	}
	if rate < 1 {
		if rand.Float32() < rate {
			value = fmt.Sprintf("%s|@%f", value, rate)
		} else {
			return nil
		}
	}

	if s.prefix != "" {
		stat = fmt.Sprintf("%s.%s", s.prefix, stat)
	}

	data := fmt.Sprintf("%s:%s", stat, value)

	_, err := s.sender.Send([]byte(data))
	if err != nil {
		return err
	}
	return nil
}

// Sets/Updates the statsd client prefix.
func (s *Client) SetPrefix(prefix string) {
	if s == nil {
		return
	}
	s.prefix = prefix
}

// SimpleSender provides a socket send interface.
type SimpleSender struct {
	// underlying connection
	c net.PacketConn
	// resolved udp address
	ra *net.UDPAddr
}

// Send sends the data to the server endpoint.
func (s *SimpleSender) Send(data []byte) (int, error) {
	// no need for locking here, as the underlying fdNet
	// already serialized writes
	n, err := s.c.(*net.UDPConn).WriteToUDP(data, s.ra)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return n, errors.New("Wrote no bytes")
	}
	return n, nil
}

// Closes SimpleSender
func (s *SimpleSender) Close() error {
	err := s.c.Close()
	return err
}

// Returns a new SimpleSender for sending to the supplied addresss.
//
// addr is a string of the format "hostname:port", and must be parsable by
// net.ResolveUDPAddr.
func NewSimpleSender(addr string) (Sender, error) {
	c, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}

	ra, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	sender := &SimpleSender{
		c:  c,
		ra: ra,
	}

	return sender, nil
}

// Returns a pointer to a new Client, and an error.
//
// addr is a string of the format "hostname:port", and must be parsable by
// net.ResolveUDPAddr.
//
// prefix is the statsd client prefix. Can be "" if no prefix is desired.
func NewClient(addr, prefix string) (Statter, error) {
	sender, err := NewSimpleSender(addr)
	if err != nil {
		return nil, err
	}

	client := &Client{
		prefix: prefix,
		sender: sender,
	}

	return client, nil
}

// Compatibility alias
var Dial = New
var New = NewClient
