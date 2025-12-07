package discovery

// Registry defines the service discovery interface
type Registry interface {
	// RegisterService registers a service instance
	RegisterService(serviceName, ip string, port uint64, metadata map[string]string) error

	// DeregisterService deregisters a service instance
	DeregisterService(serviceName, ip string, port uint64) error

	// GetService gets a healthy service instance address (host:port)
	GetService(serviceName string) (string, error)

	// GetServices gets all healthy service instance addresses
	GetServices(serviceName string) ([]string, error)

	// Subscribe subscribes to service changes
	Subscribe(serviceName string, callback func(services []string)) error

	// Close closes the registry client
	Close() error
}
