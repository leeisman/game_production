package discovery

// Registry defines the service discovery interface
type Registry interface {
	// RegisterService registers a service instance
	RegisterService(serviceName, ip string, port uint64, metadata map[string]string) error

	// DeregisterService deregisters a service instance
	DeregisterService(serviceName, ip string, port uint64) error

	// GetService gets a healthy service instance address (host:port)
	GetService(serviceName string) (string, error)

	// Close closes the registry client
	Close() error
}
