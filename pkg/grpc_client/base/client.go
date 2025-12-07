// Package base provides the base gRPC client implementation with service discovery and load balancing.
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
	pbAdmin "github.com/frankieli/game_product/shared/proto/admin"
)

// BaseClient handles gRPC connections to various services
type BaseClient struct {
	Registry discovery.Registry

	// Connections cache (Key: "ip:port")
	conns   map[string]*grpc.ClientConn
	connsMu sync.RWMutex

	// Service Discovery Cache with TTL
	serviceAddrs    map[string][]string
	serviceAddrsTTL map[string]time.Time // Still used but with long expiration
	addrsMu         sync.RWMutex
	requestGroup    singleflight.Group

	// Subscription tracking
	subscribed  map[string]bool
	subscribeMu sync.Mutex

	// Worker Pool
	broadcastQueue chan func()
}

// GetServiceAddrs returns the list of healthy instance addresses for a service.
func (c *BaseClient) GetServiceAddrs(serviceName string) ([]string, error) {
	// 1. Try Cache first
	c.addrsMu.RLock()
	addrs, ok := c.serviceAddrs[serviceName]
	c.addrsMu.RUnlock()

	// If we have addresses, return them.
	// We assume subscription keeps them up to date.
	if ok && len(addrs) > 0 {
		return addrs, nil
	}

	// 2. Fetch Initial Data & Ensure Subscription
	val, err, _ := c.requestGroup.Do(serviceName, func() (interface{}, error) {
		// Double check cache
		c.addrsMu.RLock()
		cached, ok := c.serviceAddrs[serviceName]
		c.addrsMu.RUnlock()

		if ok && len(cached) > 0 {
			return cached, nil
		}

		// Ensure we are subscribed to updates for this service
		c.ensureSubscribed(serviceName)

		// Fetch currently available instances directly (Synchronous)
		fetchedAddrs, err := c.Registry.GetServices(serviceName)
		if err != nil {
			return nil, err
		}

		// Update Cache
		c.addrsMu.Lock()
		c.serviceAddrs[serviceName] = fetchedAddrs
		c.addrsMu.Unlock()

		return fetchedAddrs, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to discover service %s: %w", serviceName, err)
	}

	return val.([]string), nil
}

// ensureSubscribed registers a listener for the service if not already registered
func (c *BaseClient) ensureSubscribed(serviceName string) {
	c.subscribeMu.Lock()
	defer c.subscribeMu.Unlock()

	if c.subscribed[serviceName] {
		return
	}

	// Register Subscription
	err := c.Registry.Subscribe(serviceName, func(services []string) {
		// Add random jitter (0-3s) to prevent thundering herd on local lock/cpu
		// when Nacos pushes updates to many clients simultaneously
		jitter := time.Duration(rand.Intn(3000)) * time.Millisecond
		time.Sleep(jitter)

		c.addrsMu.Lock()
		c.serviceAddrs[serviceName] = services
		c.addrsMu.Unlock()

		logger.InfoGlobal().
			Str("service", serviceName).
			Int("instances", len(services)).
			Msg("Service instances updated via Nacos Push")
	})

	if err != nil {
		logger.ErrorGlobal().Str("service", serviceName).Err(err).Msg("Failed to subscribe to service updates")
		// We don't mark as subscribed so we might retry later, or fallback to manual fetch
	} else {
		c.subscribed[serviceName] = true
		logger.InfoGlobal().Str("service", serviceName).Msg("Subscribed to service updates")
	}
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
		subscribed:      make(map[string]bool),
		broadcastQueue:  make(chan func(), 1024), // Buffer size 1024
	}

	// Start workers
	for i := 0; i < 20; i++ { // 20 workers
		go c.StartWorker()
	}

	return c
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

// CollectPerformance triggers performance data collection on a specific instance or random one
func (c *BaseClient) CollectPerformance(ctx context.Context, serviceName string, targetInstance string, duration int32) (*pbAdmin.CollectResp, error) {
	var conn *grpc.ClientConn
	var err error

	if targetInstance != "" {
		// Use specific instance
		conn, err = c.getConnDirect(targetInstance)
	} else {
		// Use Load Balancing (Random)
		conn, err = c.GetLBConn(serviceName)
	}

	if err != nil {
		return nil, err
	}

	client := pbAdmin.NewAdminServiceClient(conn)
	req := &pbAdmin.CollectReq{
		DurationSeconds: duration,
	}

	return client.CollectPerformanceData(c.withRequestID(ctx), req)
}
