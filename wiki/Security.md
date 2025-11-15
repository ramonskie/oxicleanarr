# Security Best Practices

This guide covers security considerations for deploying and operating OxiCleanarr in production environments.

## Table of Contents

- [Overview](#overview)
- [Authentication Security](#authentication-security)
  - [Password Management](#password-management)
  - [JWT Token Security](#jwt-token-security)
  - [Disabling Authentication](#disabling-authentication)
- [Network Security](#network-security)
  - [Reverse Proxy Setup](#reverse-proxy-setup)
  - [HTTPS/TLS Configuration](#httpstls-configuration)
  - [Firewall Configuration](#firewall-configuration)
- [File System Security](#file-system-security)
  - [Configuration File Permissions](#configuration-file-permissions)
  - [Data Directory Permissions](#data-directory-permissions)
- [Container Security](#container-security)
  - [Read-Only Volumes](#read-only-volumes)
  - [Non-Root User](#non-root-user)
  - [Capability Dropping](#capability-dropping)
- [API Security](#api-security)
  - [Rate Limiting](#rate-limiting)
  - [CORS Configuration](#cors-configuration)
  - [Input Validation](#input-validation)
- [Secret Scanning](#secret-scanning)
  - [Gitleaks Setup](#gitleaks-setup)
  - [Pre-Commit Hooks](#pre-commit-hooks)
- [Security Checklist](#security-checklist)
- [Reporting Security Issues](#reporting-security-issues)

---

## Overview

OxiCleanarr has the capability to **delete media files** and modify media libraries across multiple services. Proper security measures are **critical** to prevent unauthorized access or accidental deletion.

**Key Security Principles:**

1. **Authentication** - Protect the API with strong passwords and JWT tokens
2. **Network Security** - Use HTTPS and restrict network access
3. **File Permissions** - Limit access to configuration and data files
4. **Container Isolation** - Run in containers with minimal privileges
5. **Secret Management** - Never commit secrets to version control

**Related Pages:**
- [Configuration](Configuration.md) - Authentication configuration
- [Docker Deployment](Docker-Deployment.md) - Secure container deployment
- [API Reference](API-Reference.md) - API authentication methods

---

## Authentication Security

### Password Management

**⚠️ WARNING: Passwords are currently stored in plain text in `config.yaml`. This is a known security limitation.**

**Current Implementation:**

```yaml
admin:
  username: "admin"
  password: "your-password-here"  # Stored in plain text (insecure)
  disable_auth: false
```

**Security Considerations:**

1. **Use strong passwords:**
   - Minimum 16 characters
   - Mix of uppercase, lowercase, numbers, symbols
   - Avoid dictionary words or personal information

2. **Protect config.yaml:**
   ```bash
   # Set restrictive permissions (owner read/write only)
   chmod 600 config/config.yaml
   
   # Verify permissions
   ls -la config/config.yaml
   # Expected: -rw------- 1 user group ... config.yaml
   ```

3. **Never commit passwords to version control:**
   ```bash
   # Add to .gitignore
   echo "config/config.yaml" >> .gitignore
   ```

4. **Use environment variables (alternative):**
   ```bash
   # Set admin password via environment variable
   export ADMIN_PASSWORD="your-strong-password"
   
   # Note: Environment variable support is not yet implemented
   ```

**Future Improvements (Roadmap):**

- **Bcrypt password hashing** - Store password hashes instead of plain text
- **OAuth2/OIDC integration** - External identity providers (Authelia, Keycloak)
- **API key authentication** - Alternative to username/password

**Temporary Workaround:**

Use a reverse proxy with authentication (Traefik, Nginx, Caddy) to protect the entire OxiCleanarr instance, then disable OxiCleanarr's built-in auth:

```yaml
admin:
  disable_auth: true  # Only safe behind authenticated reverse proxy
```

### JWT Token Security

OxiCleanarr uses **JSON Web Tokens (JWT)** for API authentication after login.

**How JWT Works:**

1. User logs in with username/password → `/api/auth/login`
2. Server generates JWT token signed with secret key
3. Client sends token in `Authorization: Bearer <token>` header
4. Server validates token signature and expiration

**JWT Secret Configuration:**

```yaml
# Not yet configurable in config.yaml
# Uses default or JWT_SECRET environment variable
```

**Default Secret (INSECURE):**

```go
// utils/jwt.go:27
secret = "change-me-in-production-min-32-chars-required"
```

**⚠️ WARNING: Default secret is publicly known. Change in production!**

**Setting Custom JWT Secret:**

```bash
# Method 1: Environment variable (recommended)
export JWT_SECRET="your-random-32-char-or-longer-secret-here"
./oxicleanarr

# Method 2: Docker environment variable
docker run -e JWT_SECRET="your-secret" oxicleanarr
```

**Generating Strong JWT Secrets:**

```bash
# Method 1: OpenSSL (32 random bytes, base64 encoded)
openssl rand -base64 32

# Method 2: /dev/urandom
head -c 32 /dev/urandom | base64

# Method 3: pwgen
pwgen -s 48 1

# Example output:
# 7X9k2Lm5Np8Qr3Ts6Vw1Yz4Ac7Bf0Dg3Hj6Km9Pn2Rs5Ux8W
```

**JWT Token Expiration:**

- **Default:** 24 hours
- **Configurable:** Not yet exposed in config (hardcoded in `utils/jwt.go:33`)
- **Future:** Add `jwt_expiry` field to config.yaml

**Token Security Best Practices:**

1. **Rotate JWT secret periodically** (forces re-login for all users)
2. **Use HTTPS only** to prevent token interception
3. **Store tokens securely** in client (httpOnly cookies preferred over localStorage)
4. **Implement token refresh** (future feature) to extend sessions without storing long-lived tokens

**Implementation Details:**

- Signing method: `HS256` (HMAC with SHA-256) - `jwt.go:50`
- Token claims include: username, issued time, expiration time
- Validation checks signature and expiration - `jwt.go:55`

### Disabling Authentication

**Use Case:** Running OxiCleanarr behind an authenticated reverse proxy (Traefik, Nginx, Authelia, etc.).

**Configuration:**

```yaml
admin:
  username: "admin"       # Still required in config (not used)
  password: "password"    # Still required in config (not used)
  disable_auth: true      # Disables all authentication checks
```

**⚠️ CRITICAL WARNING: Only use `disable_auth: true` if OxiCleanarr is NOT exposed to the internet or untrusted networks.**

**Safe Scenarios:**

- ✅ Behind reverse proxy with authentication (Authelia, Keycloak)
- ✅ Firewalled internal network with VPN access only
- ✅ Local development/testing on localhost

**UNSAFE Scenarios:**

- ❌ Exposed directly to the internet
- ❌ Accessible from untrusted networks
- ❌ Multi-tenant environments without authentication

**Reverse Proxy Example (Nginx):**

```nginx
server {
    listen 443 ssl;
    server_name oxicleanarr.example.com;

    ssl_certificate /etc/ssl/certs/cert.pem;
    ssl_certificate_key /etc/ssl/private/key.pem;

    # Basic HTTP authentication
    auth_basic "OxiCleanarr Access";
    auth_basic_user_file /etc/nginx/.htpasswd;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Related Page:**
- [API Reference](API-Reference.md) - Authentication endpoint details

---

## Network Security

### Reverse Proxy Setup

**Why Use a Reverse Proxy?**

- **HTTPS/TLS termination** - Encrypt traffic between clients and server
- **Centralized authentication** - Single sign-on with Authelia, Keycloak, etc.
- **DDoS protection** - Rate limiting and request filtering
- **Load balancing** - Distribute traffic across multiple instances

**Recommended Reverse Proxies:**

| Proxy | Best For | Difficulty |
|-------|----------|------------|
| **Caddy** | Automatic HTTPS, simple config | Easy |
| **Traefik** | Docker integration, automatic service discovery | Medium |
| **Nginx** | High performance, flexible configuration | Medium |
| **Apache** | Legacy environments, extensive modules | Hard |

**Example: Caddy (Simplest)**

```caddyfile
oxicleanarr.example.com {
    reverse_proxy localhost:8080
    
    # Automatic HTTPS via Let's Encrypt
    # Optional: Basic auth
    basicauth {
        admin $2a$14$hashed_password_here
    }
}
```

**Example: Traefik (Docker)**

```yaml
# docker-compose.yml
services:
  oxicleanarr:
    image: oxicleanarr:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.oxicleanarr.rule=Host(`oxicleanarr.example.com`)"
      - "traefik.http.routers.oxicleanarr.entrypoints=websecure"
      - "traefik.http.routers.oxicleanarr.tls.certresolver=letsencrypt"
      - "traefik.http.middlewares.oxicleanarr-auth.basicauth.users=admin:$$apr1$$hashed"
      - "traefik.http.routers.oxicleanarr.middlewares=oxicleanarr-auth"
```

### HTTPS/TLS Configuration

**⚠️ ALWAYS use HTTPS in production to protect:**

- JWT tokens in Authorization headers
- Admin username/password during login
- API keys for external services (Jellyfin, Radarr, etc.)

**Obtaining SSL Certificates:**

1. **Let's Encrypt (Free, Automated):**
   ```bash
   # Using Certbot
   sudo certbot certonly --standalone -d oxicleanarr.example.com
   ```

2. **Self-Signed (Testing Only):**
   ```bash
   openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
     -keyout /etc/ssl/private/oxicleanarr.key \
     -out /etc/ssl/certs/oxicleanarr.crt
   ```

**HTTPS in Reverse Proxy (Nginx):**

```nginx
server {
    listen 443 ssl http2;
    server_name oxicleanarr.example.com;

    ssl_certificate /etc/letsencrypt/live/oxicleanarr.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/oxicleanarr.example.com/privkey.pem;
    
    # Modern SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    location / {
        proxy_pass http://localhost:8080;
        # Proxy headers
        proxy_set_header X-Forwarded-Proto https;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name oxicleanarr.example.com;
    return 301 https://$server_name$request_uri;
}
```

### Firewall Configuration

**Restrict Access to OxiCleanarr Port (8080):**

**Option 1: UFW (Ubuntu/Debian):**

```bash
# Deny external access to port 8080
sudo ufw deny 8080

# Allow only from reverse proxy host (if separate server)
sudo ufw allow from 192.168.1.10 to any port 8080

# Allow SSH
sudo ufw allow 22

# Enable firewall
sudo ufw enable
```

**Option 2: iptables:**

```bash
# Drop all incoming traffic to port 8080
sudo iptables -A INPUT -p tcp --dport 8080 -j DROP

# Allow from localhost (reverse proxy on same server)
sudo iptables -I INPUT -p tcp -s 127.0.0.1 --dport 8080 -j ACCEPT

# Save rules
sudo iptables-save > /etc/iptables/rules.v4
```

**Docker Firewall:**

```yaml
# docker-compose.yml
services:
  oxicleanarr:
    image: oxicleanarr:latest
    ports:
      - "127.0.0.1:8080:8080"  # Bind to localhost only
    networks:
      - internal  # Use internal network, not bridge

networks:
  internal:
    internal: true  # No external access
```

---

## File System Security

### Configuration File Permissions

**Protect `config.yaml` (contains plain-text passwords and API keys):**

```bash
# Set owner-only read/write permissions
chmod 600 config/config.yaml
chown oxicleanarr:oxicleanarr config/config.yaml

# Verify
ls -la config/config.yaml
# Expected: -rw------- 1 oxicleanarr oxicleanarr ... config.yaml
```

**Deny Web Server Access:**

```bash
# If running web server on same host, ensure it can't read config
sudo chown root:oxicleanarr /opt/oxicleanarr/config
sudo chmod 750 /opt/oxicleanarr/config
```

**Encrypt Configuration at Rest (Advanced):**

```bash
# Use dm-crypt/LUKS for config directory
sudo cryptsetup luksFormat /dev/sdb1
sudo cryptsetup open /dev/sdb1 oxicleanarr-config
sudo mkfs.ext4 /dev/mapper/oxicleanarr-config
sudo mount /dev/mapper/oxicleanarr-config /opt/oxicleanarr/config
```

### Data Directory Permissions

**Protect `data/` directory (contains exclusions, job history):**

```bash
# Set owner-only read/write permissions
chmod 700 data/
chown -R oxicleanarr:oxicleanarr data/

# Verify
ls -la data/
# Expected: drwx------ 2 oxicleanarr oxicleanarr ... data
```

**Files in data directory:**

- `data/exclusions.json` - User-excluded media items (not sensitive)
- `data/jobs.json` - Job execution history (not sensitive)

---

## Container Security

### Read-Only Volumes

**Mount configuration as read-only to prevent tampering:**

```yaml
# docker-compose.yml
services:
  oxicleanarr:
    image: oxicleanarr:latest
    volumes:
      - ./config:/app/config:ro  # Read-only config
      - ./data:/app/data:rw      # Read-write data (required for job storage)
```

**Benefits:**

- Prevents malicious code from modifying configuration
- Reduces attack surface if container is compromised
- Forces configuration changes via external tools (not API)

### Non-Root User

**⚠️ CRITICAL: Never run OxiCleanarr as root user.**

**Dockerfile Best Practice:**

```dockerfile
# Create non-root user
RUN addgroup -S oxicleanarr && adduser -S oxicleanarr -G oxicleanarr

# Switch to non-root user
USER oxicleanarr

# Run application
CMD ["./oxicleanarr"]
```

**Docker Compose:**

```yaml
services:
  oxicleanarr:
    image: oxicleanarr:latest
    user: "1000:1000"  # Run as specific UID:GID
```

**Verify User:**

```bash
# Check process owner
docker exec oxicleanarr ps aux
# Should NOT show root user
```

### Capability Dropping

**Drop unnecessary Linux capabilities:**

```yaml
# docker-compose.yml
services:
  oxicleanarr:
    image: oxicleanarr:latest
    cap_drop:
      - ALL  # Drop all capabilities
    cap_add:
      - NET_BIND_SERVICE  # Only if binding to port < 1024 (not needed for 8080)
    security_opt:
      - no-new-privileges:true  # Prevent privilege escalation
```

**Seccomp Profile (Advanced):**

```yaml
services:
  oxicleanarr:
    image: oxicleanarr:latest
    security_opt:
      - seccomp=seccomp-profile.json  # Custom syscall filter
```

---

## API Security

### Rate Limiting

**Note:** Rate limiting is **not yet implemented** in OxiCleanarr. Use reverse proxy rate limiting.

**Nginx Rate Limiting:**

```nginx
# Limit to 10 requests per second per IP
limit_req_zone $binary_remote_addr zone=oxicleanarr:10m rate=10r/s;

server {
    location / {
        limit_req zone=oxicleanarr burst=20 nodelay;
        proxy_pass http://localhost:8080;
    }
}
```

**Traefik Rate Limiting:**

```yaml
http:
  middlewares:
    oxicleanarr-ratelimit:
      rateLimit:
        average: 100      # Requests per second
        burst: 50         # Burst size
        period: 1m        # Time window
```

### CORS Configuration

**Note:** CORS is **not yet configurable** in OxiCleanarr. CORS headers are set in `router.go`.

**Current Implementation (Allow All Origins - Development Only):**

```go
// internal/api/router.go
c.AllowOrigins = []string{"*"}  // Insecure for production
```

**Recommended Production Configuration (Future):**

```yaml
server:
  cors:
    allowed_origins:
      - "https://oxicleanarr.example.com"
      - "https://admin.example.com"
    allowed_methods: ["GET", "POST", "PUT", "DELETE"]
    allow_credentials: true
```

### Input Validation

OxiCleanarr performs input validation on API requests to prevent:

- **SQL injection** (not applicable - no SQL database)
- **Path traversal** (file paths validated)
- **Command injection** (no shell command execution)

**Example Validation (Media Endpoints):**

- Media type must be "movie" or "tv"
- IDs must be positive integers
- Query parameters validated against whitelist

---

## Secret Scanning

### Gitleaks Setup

OxiCleanarr includes **Gitleaks configuration** to prevent committing secrets to version control.

**Configuration File:** `.gitleaks.toml:1`

**Detected Secret Types:**

| Secret Type | Pattern | Example |
|-------------|---------|---------|
| Jellyfin API Key | `jellyfin_api_key: <32-char hex>` | `a1b2c3d4e5f6...` |
| Radarr API Key | `radarr_api_key: <32-char hex>` | `f6e5d4c3b2a1...` |
| Sonarr API Key | `sonarr_api_key: <32-char hex>` | `1a2b3c4d5e6f...` |
| Jellyseerr API Key | `jellyseerr_api_key: <base64>` | `Y2hhbmdlLW1l...` |
| JWT Secret | `jwt_secret: <32+ chars>` | `change-me-in-production...` |

**Running Gitleaks Manually:**

```bash
# Install gitleaks
brew install gitleaks  # macOS
# OR
go install github.com/gitleaks/gitleaks/v8@latest  # Go

# Scan repository
gitleaks detect --source . --verbose

# Scan specific commit
gitleaks detect --commit <commit-hash>

# Scan all history
gitleaks detect --source . --log-opts="--all"
```

**Expected Output (No Secrets Found):**

```
○
    ○
    ○○
    ○○○
    ○○○○
    ○○○○○
    ○○○○○○
    ○○○○○○○
    
Finding:     0
```

### Pre-Commit Hooks

**Install pre-commit hook to scan before each commit:**

```bash
# Install pre-commit framework
pip install pre-commit

# Create .pre-commit-config.yaml
cat > .pre-commit-config.yaml << 'EOF'
repos:
  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.18.0
    hooks:
      - id: gitleaks
EOF

# Install hooks
pre-commit install

# Test hook
pre-commit run --all-files
```

**Manual Pre-Commit Script (Alternative):**

```bash
# .git/hooks/pre-commit
#!/bin/bash
gitleaks protect --staged --verbose
if [ $? -ne 0 ]; then
    echo "ERROR: Gitleaks detected secrets in staged files"
    echo "Fix the issues or use 'git commit --no-verify' to bypass (NOT RECOMMENDED)"
    exit 1
fi
```

**GitHub Actions Secret Scanning:**

```yaml
# .github/workflows/secret-scan.yml (already exists)
name: Secret Scan
on: [push, pull_request]
jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 0  # Scan entire history
      - uses: gitleaks/gitleaks-action@v2
```

**Related File:**
- `.gitleaks.toml` - Gitleaks configuration
- `.github/workflows/secret-scan.yml` - Automated secret scanning

---

## Security Checklist

Use this checklist when deploying OxiCleanarr to production:

### Authentication
- [ ] Change default admin username/password
- [ ] Use strong password (16+ characters)
- [ ] Set custom JWT secret via `JWT_SECRET` environment variable
- [ ] Consider disabling auth if behind authenticated reverse proxy

### Network
- [ ] Deploy reverse proxy (Caddy, Traefik, Nginx)
- [ ] Enable HTTPS with valid SSL certificate
- [ ] Configure firewall to block direct access to port 8080
- [ ] Use internal Docker network (not bridge mode)

### File System
- [ ] Set `chmod 600` on `config/config.yaml`
- [ ] Set `chmod 700` on `data/` directory
- [ ] Verify owner is non-root user
- [ ] Never commit `config.yaml` to version control

### Container
- [ ] Run as non-root user (UID/GID 1000 or higher)
- [ ] Mount config as read-only (`:ro`)
- [ ] Drop all Linux capabilities (`cap_drop: ALL`)
- [ ] Enable `no-new-privileges` security option

### API
- [ ] Configure reverse proxy rate limiting
- [ ] Restrict CORS origins (edit `router.go` - future config option)
- [ ] Monitor API logs for suspicious activity

### Secrets
- [ ] Run `gitleaks detect` before pushing changes
- [ ] Install pre-commit hook for automatic scanning
- [ ] Enable GitHub Actions secret scanning
- [ ] Rotate API keys periodically (Jellyfin, Radarr, Sonarr)

### Monitoring
- [ ] Set up log aggregation (ELK, Loki, etc.)
- [ ] Configure alerts for authentication failures
- [ ] Monitor disk usage (logs, data directory)
- [ ] Regular security updates (rebuild Docker image)

---

## Reporting Security Issues

**DO NOT open public GitHub issues for security vulnerabilities.**

**Responsible Disclosure:**

1. **Email:** security@oxicleanarr.dev (future - not yet active)
2. **GitHub Security Advisories:** Use "Report a vulnerability" in repository settings
3. **Encrypted Communication:** PGP key available at https://oxicleanarr.dev/pgp (future)

**What to Include:**

- Detailed description of vulnerability
- Steps to reproduce
- Potential impact assessment
- Suggested mitigation (if known)

**Expected Response Time:**

- Initial acknowledgment: 48 hours
- Severity assessment: 7 days
- Fix timeline: Depends on severity (critical: 7 days, high: 30 days, medium: 90 days)

**Security Updates:**

- Security patches released as minor versions (e.g., v1.0.1)
- Announced via GitHub Releases and security advisories
- Docker images rebuilt and tagged

---

## Additional Resources

**Related Pages:**
- [Configuration](Configuration.md) - Admin authentication setup
- [Docker Deployment](Docker-Deployment.md) - Secure container deployment
- [API Reference](API-Reference.md) - Authentication endpoints
- [Troubleshooting](Troubleshooting.md) - Authentication issues

**External Resources:**
- [OWASP Top 10](https://owasp.org/www-project-top-ten/) - Web application security risks
- [Docker Security Best Practices](https://docs.docker.com/engine/security/) - Official Docker security guide
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725) - JWT security considerations
- [Let's Encrypt](https://letsencrypt.org/) - Free SSL certificates
