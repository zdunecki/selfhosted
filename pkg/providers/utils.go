package providers

import (
	"fmt"
	"net"
	"time"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// WaitForSSH waits for SSH to become available
func WaitForSSH(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for SSH on %s", addr)
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err == nil {
				conn.Close()
				// Extra wait for SSH to fully initialize
				time.Sleep(5 * time.Second)
				return nil
			}
		}
	}
}
