package base

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/frankieli/game_product/pkg/discovery"
	"github.com/frankieli/game_product/pkg/logger"
)

// BaseClient handles gRPC connections to various services
type BaseClient struct {
	Registry discovery.Registry

	// Connections cache (Key: "ip:port")
	conns   map[string]*grpc.ClientConn
	connsMu sync.RWMutex

	// Service Discovery Cache with TTL
	serviceAddrs    map[string][]string
	serviceAddrsTTL map[string]time.Time // Cache expiration time
	addrsMu         sync.RWMutex
	requestGroup    singleflight.Group
	cacheTTL        time.Duration // Default: 10 seconds

	// Worker Pool
	broadcastQueue chan func()
}

// GetServiceAddrs returns the list of healthy instance addresses for a service.
// It uses caching with TTL, singleflight to minimize registry load.
func (c *BaseClient) GetServiceAddrs(serviceName string) ([]string, error) {
	// 1. Try Cache first and check TTL
	c.addrsMu.RLock()
	addrs, ok := c.serviceAddrs[serviceName]
	ttl, hasTTL := c.serviceAddrsTTL[serviceName]
	c.addrsMu.RUnlock()

	// Cache hit and not expired
	if ok && hasTTL && time.Now().Before(ttl) && len(addrs) > 0 {
		return addrs, nil
	}

	// 2. Cache miss or expired: Fetch from Registry with Singleflight
	val, err, _ := c.requestGroup.Do(serviceName, func() (interface{}, error) {
		// Double check cache (another goroutine might have updated it)
		c.addrsMu.RLock()
		cached, ok := c.serviceAddrs[serviceName]
		ttl, hasTTL := c.serviceAddrsTTL[serviceName]
		c.addrsMu.RUnlock()

		if ok && hasTTL && time.Now().Before(ttl) && len(cached) > 0 {
			return cached, nil
		}

		// Fetch from Registry
		fetchedAddrs, err := c.Registry.GetServices(serviceName)
		if err != nil {
			return nil, err
		}

		// Update Cache with new TTL
		c.addrsMu.Lock()
		c.serviceAddrs[serviceName] = fetchedAddrs
		c.serviceAddrsTTL[serviceName] = time.Now().Add(c.cacheTTL)
		c.addrsMu.Unlock()

		logger.InfoGlobal().
			Str("service", serviceName).
			Int("instances", len(fetchedAddrs)).
			Dur("ttl", c.cacheTTL).
			Msg("Service addresses cached")

		return fetchedAddrs, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to discover service %s: %w", serviceName, err)
	}

	return val.([]string), nil
}

// GetLBConn performs Client-Side Load Balancing (Random) to get a connection for a service
func (c *BaseClient) GetLBConn(serviceName string) (*grpc.ClientConn, error) {
	addrs, err := c.GetServiceAddrs(serviceName)
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no instances found for service %s", serviceName)
	}

	// 3. Pick Random Instance
	idx := rand.Intn(len(addrs))
	targetAddr := addrs[idx]

	// 4. Get Persistent Connection
	return c.getConnDirect(targetAddr)
}

// NewBaseClient creates a new unified gRPC client manager
func NewBaseClient(registry discovery.Registry) *BaseClient {
	c := &BaseClient{
		Registry:        registry,
		conns:           make(map[string]*grpc.ClientConn),
		serviceAddrs:    make(map[string][]string),
		serviceAddrsTTL: make(map[string]time.Time),
		cacheTTL:        10 * time.Second,        // Cache TTL: 10 seconds
		broadcastQueue:  make(chan func(), 1024), // Buffer size 1024
	}

	// Start workers
	for i := 0; i < 20; i++ { // 20 workers
		go c.StartWorker()
	}

	// Start Service Watcher
	go c.StartServiceWatcher()

	return c
}

// StartServiceWatcher periodically updates the service cache for all known services
func (c *BaseClient) StartServiceWatcher() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.addrsMu.RLock()
		if len(c.serviceAddrs) == 0 {
			c.addrsMu.RUnlock()
			continue
		}

		// Collect known services
		services := make([]string, 0, len(c.serviceAddrs))
		for k := range c.serviceAddrs {
			services = append(services, k)
		}
		c.addrsMu.RUnlock()

		c.updateServices(services...)
	}
}

func (c *BaseClient) updateServices(services ...string) {
	for _, svc := range services {
		addrs, err := c.Registry.GetServices(svc)
		if err != nil {
			logger.WarnGlobal().Str("service", svc).Err(err).Msg("Failed to update service cache")
			continue
		}

		c.addrsMu.Lock()
		c.serviceAddrs[svc] = addrs
		c.addrsMu.Unlock()
	}
}

func (c *BaseClient) StartWorker() {
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorGlobal().Msgf("Worker panic: %v", r)
			go c.StartWorker() // Restart worker
		}
	}()

	for task := range c.broadcastQueue {
		task()
	}
}

func (c *BaseClient) SubmitTask(task func()) {
	select {
	case c.broadcastQueue <- task:
		// Task submitted to pool
	default:
		// Fallback: spawn goroutine if pool is full
		logger.WarnGlobal().Msg("Broadcast worker pool full, spawning ephemeral goroutine")
		go task()
	}
}

// getConnDirect gets or creates a persistent connection to a specific address
func (c *BaseClient) getConnDirect(addr string) (*grpc.ClientConn, error) {
	c.connsMu.RLock()
	conn, ok := c.conns[addr]
	c.connsMu.RUnlock()
	if ok {
		return conn, nil
	}

	c.connsMu.Lock()
	defer c.connsMu.Unlock()

	// Double check
	if conn, ok := c.conns[addr]; ok {
		return conn, nil
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial addr %s: %w", addr, err)
	}

	c.conns[addr] = conn
	return conn, nil
}

// GetConn maintains backward compatibility but now uses Load Balancing logic
func (c *BaseClient) GetConn(serviceName string) (*grpc.ClientConn, error) {
	return c.GetLBConn(serviceName)
}

// Close closes all connections
func (c *BaseClient) Close() error {
	c.connsMu.Lock()
	defer c.connsMu.Unlock()

	for _, conn := range c.conns {
		conn.Close()
	}
	return nil
}

// withRequestID adds the request ID from the context to the gRPC metadata
func (c *BaseClient) withRequestID(ctx context.Context) context.Context {
	reqID := logger.GetRequestID(ctx)
	if reqID != "" {
		// Create new metadata with request_id and append to context
		// Note: AppendToOutgoingContext creates a new context
		return metadata.AppendToOutgoingContext(ctx, "request_id", reqID)
	}
	return ctx
}
