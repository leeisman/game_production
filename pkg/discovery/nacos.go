package discovery

import (
	"fmt"
	"strconv"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// NacosClient wraps Nacos naming client
type NacosClient struct {
	client naming_client.INamingClient
}

// NewNacosClient creates a new Nacos client
func NewNacosClient(host string, port string, namespaceID string) (*NacosClient, error) {
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	serverConfigs := []constant.ServerConfig{
		{
			IpAddr: host,
			Port:   uint64(portInt),
		},
	}

	clientConfig := constant.ClientConfig{
		NamespaceId:         namespaceID,
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		LogLevel:            "info",
	}

	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos client: %w", err)
	}

	return &NacosClient{client: namingClient}, nil
}

// RegisterService registers a service instance to Nacos
func (nc *NacosClient) RegisterService(serviceName, ip string, port uint64, metadata map[string]string) error {
	success, err := nc.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        port,
		ServiceName: serviceName,
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}
	if !success {
		return fmt.Errorf("register service returned false")
	}
	return nil
}

// DeregisterService deregisters a service instance from Nacos
func (nc *NacosClient) DeregisterService(serviceName, ip string, port uint64) error {
	success, err := nc.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          ip,
		Port:        port,
		ServiceName: serviceName,
	})
	if err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}
	if !success {
		return fmt.Errorf("deregister service returned false")
	}
	return nil
}

// GetService gets a healthy service instance from Nacos
func (nc *NacosClient) GetService(serviceName string) (string, error) {
	instance, err := nc.client.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
		ServiceName: serviceName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get service instance: %w", err)
	}

	return fmt.Sprintf("%s:%d", instance.Ip, instance.Port), nil
}

// Close closes the Nacos client
func (nc *NacosClient) Close() error {
	// Nacos SDK doesn't provide explicit close method
	return nil
}
