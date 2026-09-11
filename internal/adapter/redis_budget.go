package adapter

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
)

// Validate RESP lengths before the driver can allocate the declared payload.
// A byte-counting reader alone cannot bound allocations from an oversized header.
type readBudgetConn struct {
	net.Conn
	reader    *bufio.Reader
	remaining int
	entries   int
	bulk      int64
	pending   []byte
}

func newReadBudgetConn(c net.Conn, limit int) *readBudgetConn {
	return &readBudgetConn{Conn: c, reader: bufio.NewReaderSize(c, 4096), remaining: limit, entries: limit / 16}
}
func (c *readBudgetConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if c.remaining <= 0 {
		return 0, errors.New("database response exceeds byte budget")
	}
	if len(p) > c.remaining {
		p = p[:c.remaining]
	}
	if len(c.pending) > 0 {
		n := copy(p, c.pending)
		c.pending = c.pending[n:]
		c.remaining -= n
		return n, nil
	}
	if c.bulk > 0 {
		if int64(len(p)) > c.bulk {
			p = p[:c.bulk]
		}
		n, e := c.reader.Read(p)
		c.bulk -= int64(n)
		c.remaining -= n
		return n, e
	}
	line, e := c.reader.ReadSlice('\n')
	if e != nil {
		return 0, e
	}
	if len(line) < 3 || line[len(line)-2] != '\r' {
		return 0, errors.New("invalid RESP header")
	}
	switch line[0] {
	case '$', '*', '%', '~', '>':
		n, e := strconv.ParseInt(string(line[1:len(line)-2]), 10, 64)
		if e != nil || n < -1 {
			return 0, errors.New("invalid RESP length")
		}
		if line[0] == '$' && n >= 0 {
			if n > int64(c.remaining-len(line)-2) {
				return 0, errors.New("database bulk value exceeds byte budget")
			}
			c.bulk = n + 2
		} else if n > 0 {
			if line[0] == '%' {
				if n > int64(c.entries/2) {
					return 0, errors.New("database map exceeds allocation budget")
				}
				n *= 2
			}
			if n > int64(c.entries) {
				return 0, errors.New("database array exceeds allocation budget")
			}
			c.entries -= int(n)
		}
	case '+', '-', ':', ',', '_', '#':
	default:
		return 0, errors.New("unsupported RESP reply")
	}
	if len(line) > c.remaining {
		return 0, io.ErrShortBuffer
	}
	c.pending = line
	return c.Read(p)
}
