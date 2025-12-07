// Package netutil provides network utility functions such as getting outbound IP and listening on ports.
package netutil

import (
	"fmt"
	"net"
)

// GetOutboundIP gets the preferred outbound ip of this machine
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

// ListenWithFallback attempts to listen on the preferred port.
// If it fails (e.g., port in use), it falls back to a random available system port (:0).
// It returns the listener and the actual port selected.
func ListenWithFallback(preferredPort string) (net.Listener, int, error) {
	// 1. Try preferred port
	lis, err := net.Listen("tcp", ":"+preferredPort)
	if err == nil {
		addr := lis.Addr().(*net.TCPAddr)
		return lis, addr.Port, nil
	}

	// 2. Fallback to random port
	// Check if error is "address already in use" is implicitly handled by just trying fallback
	// You might want to log the error in the caller
	lis, err = net.Listen("tcp", ":0")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to listen on preferred port %s and random port: %w", preferredPort, err)
	}

	addr := lis.Addr().(*net.TCPAddr)
	return lis, addr.Port, nil
}
