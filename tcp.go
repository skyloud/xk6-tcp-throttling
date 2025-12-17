package tcp

import (
	"net"
	"syscall"
	"time"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/tcp-throttling", new(TCP))
}

type TCP struct{}

type Connection struct {
	conn net.Conn
	throttler *BandwidthThrottler
}

// BandwidthThrottler implements bandwidth limiting
// Inspired by https://github.com/boz/go-throttle/blob/master/throttle.go
type BandwidthThrottler struct {
	bytesPerSecond int
	lastRead       time.Time
	bytesRead      int
}

func (t *TCP) Connect(addr string) *Connection {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	return &Connection{conn: conn}
}

func (c *Connection) SetReceiveBuffer(size int) error {
	if tcpConn, ok := c.conn.(*net.TCPConn); ok {
		rawConn, err := tcpConn.SyscallConn()
		if err != nil {
			return err
		}
		return rawConn.Control(func(fd uintptr) {
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, size)
		})
	}
	return nil
}

func (c *Connection) SetBandwidthLimit(bytesPerSecond int) {
	c.throttler = &BandwidthThrottler{
		bytesPerSecond: bytesPerSecond,
		lastRead:       time.Now(),
	}
}

func (c *Connection) ReadWithThrottle(size int) []byte {
	buffer := make([]byte, size)
	totalRead := 0
	
	for totalRead < size {
		// Lire par petits chunks pour permettre le throttling progressif
		chunkSize := 4096
		if size-totalRead < chunkSize {
			chunkSize = size - totalRead
		}
		
		// Appliquer le throttling AVANT de lire
		if c.throttler != nil {
			c.throttler.waitIfNeeded(chunkSize)
		}
		
		n, err := c.conn.Read(buffer[totalRead:totalRead+chunkSize])
		if err != nil {
			if totalRead > 0 {
				break
			}
			panic(err)
		}
		
		if c.throttler != nil {
			c.throttler.recordRead(n)
		}
		
		totalRead += n
		if n == 0 {
			break
		}
	}
	
	return buffer[:totalRead]
}

func (c *Connection) ReadWithDelay(size int, delayMs int) []byte {
	buffer := make([]byte, size)
	n, err := c.conn.Read(buffer)
	if err != nil {
		panic(err)
	}
	
	time.Sleep(time.Duration(delayMs) * time.Millisecond)
	return buffer[:n]
}

func (t *BandwidthThrottler) waitIfNeeded(bytesToRead int) {
	now := time.Now()
	elapsed := now.Sub(t.lastRead)
	
	// Reset le compteur chaque seconde
	if elapsed >= time.Second {
		t.bytesRead = 0
		t.lastRead = now
		return
	}
	
	// Si on dépasse la limite, attendre
	if t.bytesRead+bytesToRead > t.bytesPerSecond {
		sleepTime := time.Second - elapsed
		time.Sleep(sleepTime)
		t.bytesRead = 0
		t.lastRead = time.Now()
	}
}

func (t *BandwidthThrottler) recordRead(bytesRead int) {
	t.bytesRead += bytesRead
}

func (c *Connection) Write(data []byte) {
	c.conn.Write(data)
}

func (c *Connection) Close() {
	c.conn.Close()
}