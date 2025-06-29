package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

type ProxyConfig struct {
	Name       string `yaml:"name"`
	LocalHost  string `yaml:"local_host"`
	LocalPort  int    `yaml:"local_port"`
	RemoteHost string `yaml:"remote_host"`
	RemotePort int    `yaml:"remote_port"`
	Enabled    bool   `yaml:"enabled"`
}

type Config struct {
	Proxies []ProxyConfig `yaml:"proxies"`
}

type ProxyManager struct {
	config  *Config
	proxies map[string]*Proxy
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
}

type Proxy struct {
	config   ProxyConfig
	listener net.Listener
	active   bool
	ctx      context.Context
	cancel   context.CancelFunc
}

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/config/proxies.yml"
	}

	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &ProxyManager{
		config:  config,
		proxies: make(map[string]*Proxy),
		ctx:     ctx,
		cancel:  cancel,
	}

	if err := manager.startProxies(); err != nil {
		log.Fatalf("Failed to start proxies: %v", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Println("TCP Proxy Manager started successfully")
	log.Println("Active proxies:")
	for _, proxy := range config.Proxies {
		if proxy.Enabled {
			log.Printf("  %s: %s:%d -> %s:%d",
				proxy.Name, proxy.LocalHost, proxy.LocalPort,
				proxy.RemoteHost, proxy.RemotePort)
		}
	}

	<-c
	log.Println("Shutting down...")
	manager.shutdown()
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	setDefaults(&config)
	return &config, nil
}

func setDefaults(config *Config) {
	for i := range config.Proxies {
		if config.Proxies[i].LocalHost == "" {
			config.Proxies[i].LocalHost = "0.0.0.0"
		}
	}
}

func (pm *ProxyManager) startProxies() error {
	for _, proxyConfig := range pm.config.Proxies {
		if !proxyConfig.Enabled {
			log.Printf("Skipping disabled proxy: %s", proxyConfig.Name)
			continue
		}

		if err := pm.startProxy(proxyConfig); err != nil {
			return fmt.Errorf("failed to start proxy %s: %w", proxyConfig.Name, err)
		}
	}
	return nil
}

func (pm *ProxyManager) startProxy(config ProxyConfig) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	localAddr := fmt.Sprintf("%s:%d", config.LocalHost, config.LocalPort)
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", localAddr, err)
	}

	ctx, cancel := context.WithCancel(pm.ctx)

	proxy := &Proxy{
		config:   config,
		listener: listener,
		active:   true,
		ctx:      ctx,
		cancel:   cancel,
	}

	pm.proxies[config.Name] = proxy

	pm.wg.Add(1)
	go pm.handleProxy(proxy)

	log.Printf("Started proxy %s: %s -> %s:%d",
		config.Name, localAddr, config.RemoteHost, config.RemotePort)

	return nil
}

func (pm *ProxyManager) handleProxy(proxy *Proxy) {
	defer pm.wg.Done()
	defer proxy.listener.Close()

	for {
		select {
		case <-proxy.ctx.Done():
			return
		default:
		}

		conn, err := proxy.listener.Accept()
		if err != nil {
			select {
			case <-proxy.ctx.Done():
				return
			default:
				log.Printf("Failed to accept connection for proxy %s: %v", proxy.config.Name, err)
				continue
			}
		}

		go pm.handleConnection(proxy, conn)
	}
}

func (pm *ProxyManager) handleConnection(proxy *Proxy, clientConn net.Conn) {
	defer clientConn.Close()

	remoteAddr := fmt.Sprintf("%s:%d", proxy.config.RemoteHost, proxy.config.RemotePort)

	var serverConn net.Conn
	var err error

	for attempt := 1; attempt <= 3; attempt++ {
		dialer := &net.Dialer{
			Timeout: 10 * time.Second,
		}
		serverConn, err = dialer.Dial("tcp", remoteAddr)
		if err == nil {
			break
		}

		log.Printf("Connection attempt %d to %s failed: %v", attempt, remoteAddr, err)
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	if err != nil {
		log.Printf("Failed to connect to remote server %s for proxy %s after 3 attempts: %v",
			remoteAddr, proxy.config.Name, err)
		return
	}
	defer serverConn.Close()

	log.Printf("Established connection for proxy %s: %s -> %s",
		proxy.config.Name, clientConn.RemoteAddr(), remoteAddr)

	ctx, cancel := context.WithCancel(proxy.ctx)
	defer cancel()

	go func() {
		defer cancel()
		_, err := io.Copy(serverConn, clientConn)
		if err != nil {
			log.Printf("Error copying client->server for proxy %s: %v", proxy.config.Name, err)
		}
	}()

	go func() {
		defer cancel()
		_, err := io.Copy(clientConn, serverConn)
		if err != nil {
			log.Printf("Error copying server->client for proxy %s: %v", proxy.config.Name, err)
		}
	}()

	<-ctx.Done()
}

func (pm *ProxyManager) shutdown() {
	log.Println("Shutting down proxy manager...")

	pm.cancel()

	pm.mu.Lock()
	for name, proxy := range pm.proxies {
		log.Printf("Closing proxy: %s", name)
		proxy.cancel()
	}
	pm.mu.Unlock()

	pm.wg.Wait()

	log.Println("Proxy manager shutdown complete")
}
