# xk6-tcp-throttle

K6 extension for TCP connections with bandwidth throttling and ACK delay control.

## Features

- TCP connection with receive buffer control
- Bandwidth throttling (bytes per second)
- ACK delay simulation for natural bandwidth throttling
- Compatible with K6 load testing

## Installation

```bash
xk6 build --with github.com/skyloud/xk6-tcp-throttling
```

## Usage

```javascript
import tcpThrottle from 'k6/x/tcp-throttling';

export default function() {
  const conn = tcpThrottle.connect('server:80');
  
  // Method 1: Bandwidth throttling (1MB/s)
  conn.setBandwidthLimit(1024 * 1000);
  const data1 = conn.readWithThrottle(8192);
  
  // Method 2: ACK delay simulation
  const data2 = conn.readWithDelay(8192, 100);
  
  // Method 3: TCP window control
  conn.setReceiveBuffer(1024);
  
  conn.close();
}
```

## API

- `connect(addr)` - Create TCP connection
- `setBandwidthLimit(bytesPerSecond)` - Set bandwidth limit
- `readWithThrottle(size)` - Read with automatic bandwidth throttling
- `setReceiveBuffer(size)` - Set SO_RCVBUF socket option
- `readWithDelay(size, delayMs)` - Read data with ACK delay
- `write(data)` - Write data
- `close()` - Close connection

## Credits

Bandwidth throttling implementation inspired by [go-throttle](https://github.com/boz/go-throttle/blob/master/throttle.go)