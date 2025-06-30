# TCP Proxy Service

This service provides direct TCP port forwarding through VPN connections. It allows you to easily access private databases and services behind a VPN using only your OpenVPN configuration.

## Project Structure

```
tcp-proxy/
├── .github/
│   └── workflows/
│       └── docker-build.yml       # CI/CD pipeline
├── scripts/
│   └── release.sh                 # Release helper script
├── config/
│   ├── gluetun/                   # VPN configuration
│   │   ├── config.ovpn            # Your OpenVPN configuration
│   │   ├── auth.txt               # VPN credentials (create from example)
│   │   └── auth.txt.example       # Template for VPN credentials
│   └── tcp-proxy/
│       ├── proxies.yml            # Proxy configuration (create from example)
│       └── proxies.yml.example    # Template for proxy configuration
├── main.go                        # TCP proxy implementation
├── go.mod                         # Go module dependencies
├── Dockerfile                     # Multi-platform Docker build
├── docker-compose.yml             # Service orchestration
└── README.md                      # This file
```

## Features

- Simple TCP port forwarding through VPN
- Dynamic proxy configuration via YAML
- VPN connection through Gluetun (OpenVPN support)
- Automatic reconnection with retry logic (3 attempts with exponential backoff)
- Support for multiple simultaneous proxies
- Concurrent connection handling with goroutines
- Graceful shutdown with proper cleanup
- Comprehensive connection logging and error handling
- 10-second connection timeout per attempt
- No SSH keys required - only OpenVPN config needed
- Integrated with existing homelab network stack
- Built with Go 1.24.4 for performance and reliability

## How It Works

```
Your App → localhost:15432 → TCP Proxy → VPN → Remote PostgreSQL:5432
Your App → localhost:16379 → TCP Proxy → VPN → Remote Redis:6379
```

The TCP proxy service runs inside the same Docker network as Gluetun, so all connections automatically go through your VPN.

## Setup

### 1. Docker Image

The image is available on Docker Hub: **`phathdt379/tcp-proxy:latest`**

The docker-compose.yml is already configured to use this image from Docker Hub, so no local building is required.

Features:
- Built with Go 1.24.4 using multi-stage Docker build
- Minimal distroless base image (only 4.82MB)
- Runs as non-root user for security
- Support for both AMD64 and ARM64 architectures
- Static binary with optimized build flags

### 2. Place Your OpenVPN Configuration

Put your `.ovpn` file in `config/gluetun/config.ovpn`

If your VPN requires authentication, create `config/gluetun/auth.txt`:
```
username
password
```

💡 **Tip:** Use the provided example files as templates:
- Copy `config/gluetun/auth.txt.example` to `config/gluetun/auth.txt` and edit with your credentials
- Copy `config/tcp-proxy/proxies.yml.example` to `config/tcp-proxy/proxies.yml` and configure your proxies

#### OpenVPN Configuration for Containers

Most standard OpenVPN configurations need modifications to work properly in containerized environments with gluetun. Here are the common changes required:

**⚠️ Important:** Your original `.ovpn` file may need these modifications:

1. **Remove `auth-nocache` directive** (if present):
   ```diff
   - auth-nocache
   ```
   *Reason: Credential caching can cause issues in containers. gluetun handles authentication through the `auth.txt` file.*

2. **Add authentication file reference**:
   ```diff
   + askpass /config/auth.txt
   ```
   *Reason: This tells OpenVPN where to find your username/password file in the container.*

3. **Remove conflicting route directives** (if present):
   ```diff
   - setenv opt block-outside-dns # Prevent Windows 10 DNS leak
   ```
   *Reason: Container networking and gluetun handle DNS and routing automatically.*

4. **IPv6 routes are automatically filtered** by docker-compose configuration:
   ```yaml
   OPENVPN_FLAGS=--pull-filter ignore "route-ipv6" --pull-filter ignore "ifconfig-ipv6"
   ```
   *Reason: IPv6 can cause connectivity issues in Docker containers.*

**Example of a typical modification:**

**Original `.ovpn` file:**
```
client
dev tun
proto udp
remote vpn.example.com 1194
auth SHA256
cipher AES-128-GCM
auth-nocache                     # ← Remove this line
tls-client
setenv opt block-outside-dns     # ← Remove this line
```

**Modified for container use:**
```
client
dev tun
proto udp
remote vpn.example.com 1194
auth SHA256
cipher AES-128-GCM
askpass /config/auth.txt           # ← Add this line for authentication
tls-client
```

**💡 Pro Tip:** If your VPN connection fails, check gluetun logs:
```bash
docker-compose logs gluetun
```

Common issues are usually related to:
- Incompatible OpenVPN directives for containers
- DNS configuration conflicts
- Route conflicts with Docker networking

### 3. Configure Proxies

Create your proxy configuration from the example template:

```bash
# Copy the example and edit with your settings
cp config/tcp-proxy/proxies.yml.example config/tcp-proxy/proxies.yml
```

The configuration file path can be customized using the `CONFIG_PATH` environment variable (default: `/config/proxies.yml`).

```yaml
proxies:
  - name: "postgres-main"
    local_host: "0.0.0.0"         # Listen on all interfaces
    local_port: 15432             # Local port to bind
    remote_host: "10.0.1.10"      # Remote server IP (accessible through VPN)
    remote_port: 5432             # Remote server port
    enabled: true

  - name: "redis-cache"
    local_host: "0.0.0.0"
    local_port: 16379
    remote_host: "10.0.1.20"
    remote_port: 6379
    enabled: true
```

### 4. Deploy

Start the services:
```bash
# Start the services
docker-compose up -d
```

The tcp-proxy service will automatically start after gluetun is ready and share its network stack.

## Usage

Once deployed, you can connect to your services through the proxies:

### PostgreSQL Connections
```bash
# Connect to the main PostgreSQL through proxy
psql -h localhost -p 15432 -U your_username -d your_database

# Connect to the secondary PostgreSQL through proxy
psql -h localhost -p 15433 -U your_username -d your_database
```

### Redis Connections
```bash
# Connect to the main Redis through proxy
redis-cli -h localhost -p 16379

# Connect to the Redis sessions through proxy
redis-cli -h localhost -p 16380
```

### Application Configuration

Configure your applications to connect to:
- **PostgreSQL Main**: `localhost:15432`
- **Redis Cache**: `localhost:16379`
- **PostgreSQL Secondary**: `localhost:15433`
- **Redis Sessions**: `localhost:16380`

## Adding New Proxies

The current setup provides 4 proxy ports by default: `15432`, `16379`, `15433`, `16380`.

1. Edit `config/tcp-proxy/proxies.yml`
2. Add a new proxy configuration using one of the available ports:
   ```yaml
   - name: "postgres-secondary"
     local_host: "0.0.0.0"
     local_port: 15433
     remote_host: "remote.server.ip"
     remote_port: 5432
     enabled: true
   ```
3. To add more ports beyond the default 4, update `docker-compose.yml`:
   - Add the port to the gluetun ports section:
     ```yaml
     ports:
       - '17000:17000/tcp'
     ```
   - Update the firewall input ports:
     ```yaml
     - FIREWALL_INPUT_PORTS=8888,15432,16379,15433,16380,17000
     ```
4. Restart the services:
   ```bash
   docker-compose restart gluetun tcp-proxy
   ```

## Building and Deployment

### Automated CI/CD

The project includes GitHub Actions for automated building and deployment:

- **Trigger**: Pushing a new tag (e.g., `v1.0.0`)
- **Platforms**: Builds for `linux/amd64` and `linux/arm64` (Mac M1 support)
- **Registry**: Pushes to Docker Hub as `phathdt379/tcp-proxy`
- **Tags**: Creates both version tag and `latest` tag

To create a new release:

```bash
# Using the release script (recommended)
./scripts/release.sh v1.0.0

# Or manually
git tag v1.0.0
git push origin v1.0.0
```

This will automatically trigger the CI/CD pipeline to build and push the image.

### Manual Building

If you need to build the image manually:

```bash
# Build for local platform
docker build -t phathdt379/tcp-proxy:latest .

# Build for multiple platforms (requires buildx)
docker buildx build --platform linux/amd64,linux/arm64 -t phathdt379/tcp-proxy:latest --push .
```

### Docker Hub Setup

To use the automated CI/CD, you need to set up these GitHub secrets:

- `DOCKERHUB_USERNAME`: Your Docker Hub username
- `DOCKERHUB_TOKEN`: Your Docker Hub access token (not password)

## Monitoring

### Check Logs
```bash
# View proxy service logs
docker logs tcp-proxy

# View VPN logs
docker logs gluetun
```

### Health Check
```bash
# Check if proxies are listening
netstat -tlnp | grep -E '(15432|16379|15433|16380)'

# Test connections
telnet localhost 15432  # PostgreSQL main
telnet localhost 16379  # Redis cache
telnet localhost 15433  # PostgreSQL secondary
telnet localhost 16380  # Redis sessions
```

### VPN Status
Visit `http://vpn.yourdomain.local` to check VPN status through Traefik.

## Troubleshooting

### VPN Connection Issues
1. Check your `.ovpn` file is correctly placed in `config/gluetun/config.ovpn`
2. Verify authentication credentials in `config/gluetun/auth.txt` if required
3. Check Gluetun logs: `docker logs gluetun`

### Proxy Connection Issues
1. Verify remote services are running and accessible through VPN
2. Check if ports are correctly forwarded
3. Test with: `telnet localhost PORT`
4. Check proxy logs: `docker logs tcp-proxy`

### Remote Service Access
1. Make sure your VPN gives you access to the remote network
2. Test VPN connectivity: `docker exec gluetun ping remote.server.ip`
3. Verify remote service is listening on expected port

### Docker Image Issues
1. Rebuild the image: `./scripts/build-tcp-proxy.sh`
2. Check image exists: `docker images | grep tcp-proxy`
3. Pull latest if using from Docker Hub: `docker pull phathdt379/tcp-proxy:latest`

### Configuration Validation
```bash
# Validate YAML syntax
docker run --rm -v $(pwd)/config/tcp-proxy:/config alpine/yamllint /config/proxies.yml
```

## Example Configurations

### Multiple PostgreSQL Databases
```yaml
proxies:
  - name: "postgres-main"
    local_host: "0.0.0.0"
    local_port: 15432
    remote_host: "10.0.1.10"
    remote_port: 5432
    enabled: true

  - name: "postgres-test"
    local_host: "0.0.0.0"
    local_port: 15433
    remote_host: "10.0.1.11"
    remote_port: 5432
    enabled: true
```

### Multiple Services Using All Available Ports
```yaml
proxies:
  - name: "postgres-main"
    local_host: "0.0.0.0"
    local_port: 15432
    remote_host: "10.0.1.10"
    remote_port: 5432
    enabled: true

  - name: "redis-cache"
    local_host: "0.0.0.0"
    local_port: 16379
    remote_host: "10.0.1.20"
    remote_port: 6379
    enabled: true

  - name: "postgres-secondary"
    local_host: "0.0.0.0"
    local_port: 15433
    remote_host: "10.0.1.11"
    remote_port: 5432
    enabled: true

  - name: "redis-sessions"
    local_host: "0.0.0.0"
    local_port: 16380
    remote_host: "10.0.1.21"
    remote_port: 6379
    enabled: true
```

## Technical Details

### Implementation
- Written in Go using standard library for maximum performance
- Uses `gopkg.in/yaml.v3` for configuration parsing
- Implements proper context-based cancellation for graceful shutdown
- Connection multiplexing using goroutines for each client connection
- Bidirectional data copying using `io.Copy`

### Retry Logic
- 3 connection attempts with exponential backoff (1s, 2s delays)
- 10-second timeout per connection attempt
- Detailed logging for connection failures and successes

### Resource Management
- Proper cleanup of network listeners and connections
- Context-based cancellation propagation
- WaitGroup synchronization for graceful shutdown

## Security Notes

- All traffic is routed through your VPN connection
- No additional authentication required beyond VPN access
- Monitor connection logs for suspicious activity
- Keep VPN client updated
- Use strong VPN credentials
- Docker image runs as non-root user (nobody:nobody) in Alpine Linux
