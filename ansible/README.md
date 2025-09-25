# ByteFreezer Piper Ansible Deployment

This directory contains Ansible playbooks for deploying ByteFreezer Piper service.

## Prerequisites

1. **Ansible installed** on the control machine
2. **Target servers** with SSH access configured
3. **PostgreSQL database** accessible (can be installed separately)

## Playbooks Overview

### Main Installation Playbooks

- **`install.yml`** - Main installation from GitHub releases (recommended)
- **`docker_install.yml`** - Docker-based installation
- **`local_install.yml`** - Local binary installation
- **`remove.yml`** - Uninstall ByteFreezer Piper service

### Database Setup

- **`postgresql_install.yml`** - Install and configure PostgreSQL server

## Quick Start

### 1. Install PostgreSQL (if needed)

If you need to install PostgreSQL on the same server as ByteFreezer Piper:

```bash
# Install Ansible requirements
ansible-galaxy install -r requirements.yml

# Install PostgreSQL
ansible-playbook -i inventory postgresql_install.yml
```

### 2. Configure Variables

Edit `group_vars/all.yml` to configure:

- PostgreSQL connection settings
- S3 endpoints and credentials
- Service configuration

### 3. Install ByteFreezer Piper

```bash
# Standard installation from GitHub releases
ansible-playbook -i inventory install.yml

# Or with specific version
ansible-playbook -i inventory install.yml -e bytefreezer_piper_version=v1.0.0
```

## Configuration

### PostgreSQL Configuration

The service requires PostgreSQL for state management. Configure in `group_vars/all.yml`:

```yaml
config:
  postgresql:
    host: "192.168.86.137"    # PostgreSQL server IP
    port: 5432
    database: "bytefreezer"
    username: "bytefreezer"
    password: "bytefreezer"
    ssl_mode: "disable"
    schema: "public"
```

### Service Dependencies

The systemd service file has been updated to **not** require a local PostgreSQL service, allowing for:
- Remote PostgreSQL databases
- Different PostgreSQL service names across distributions
- Graceful handling of temporary database unavailability

## Troubleshooting

### Service Won't Start

1. **Check service status:**
   ```bash
   systemctl status bytefreezer-piper
   ```

2. **Check logs:**
   ```bash
   journalctl -u bytefreezer-piper -f
   ```

3. **Test PostgreSQL connectivity:**
   ```bash
   nc -zv <postgresql-host> 5432
   ```

### PostgreSQL Connection Issues

If you see "Unit postgresql.service not found":

1. **PostgreSQL is remote** - This is normal, the service should start anyway
2. **PostgreSQL not installed locally** - Run `postgresql_install.yml` if you need local PostgreSQL
3. **Different service name** - Some distributions use `postgresql@14-main.service` or `postgresql-14.service`

### Common Error Messages

**"Unit postgresql.service not found"**
- This error occurred because the old systemd service had a hard dependency on `postgresql.service`
- **Fixed in current version** - systemd service no longer requires local PostgreSQL
- **Solution**: Re-run the install playbook to get the updated service file

**"Connection refused" to PostgreSQL**
- Check if PostgreSQL is running: `systemctl status postgresql`
- Verify host/port configuration in `group_vars/all.yml`
- Test network connectivity: `telnet <host> <port>`

**"Authentication failed" to PostgreSQL**
- Verify username/password in configuration
- Check PostgreSQL pg_hba.conf allows connections
- Ensure database and user exist

## Installation Methods

### Method 1: GitHub Releases (Recommended)
```bash
ansible-playbook -i inventory install.yml
```
Downloads the latest released binary.

### Method 2: Docker
```bash
ansible-playbook -i inventory docker_install.yml
```
Runs ByteFreezer Piper in a Docker container.

### Method 3: Local Binary
```bash
ansible-playbook -i inventory local_install.yml
```
Uses a locally built binary.

## File Locations

After installation:

- **Binary**: `/usr/local/bin/bytefreezer-piper`
- **Config**: `/etc/bytefreezer-piper/config.yaml`
- **Logs**: `/var/log/bytefreezer-piper/`
- **Cache**: `/var/cache/bytefreezer-piper/`
- **Spool**: `/var/spool/bytefreezer-piper/`
- **Service**: `/etc/systemd/system/bytefreezer-piper.service`

## Service Management

```bash
# Start service
sudo systemctl start bytefreezer-piper

# Stop service
sudo systemctl stop bytefreezer-piper

# Restart service
sudo systemctl restart bytefreezer-piper

# Enable auto-start
sudo systemctl enable bytefreezer-piper

# Check status
sudo systemctl status bytefreezer-piper

# View logs
sudo journalctl -u bytefreezer-piper -f
```

## Health Monitoring

Once installed, the service provides monitoring endpoints:

- **Health Check**: `http://<server>:8080/health`
- **Metrics**: `http://<server>:9090/metrics`

## Uninstallation

```bash
# Remove service and files
ansible-playbook -i inventory remove.yml

# Remove cache and spool directories too
ansible-playbook -i inventory remove.yml -e bytefreezer_piper_remove_cache_dir=true -e bytefreezer_piper_remove_spool_dir=true
```

## Advanced Configuration

### Custom Variables

Override default variables by setting them in your inventory or command line:

```bash
# Use specific version
ansible-playbook install.yml -e bytefreezer_piper_version=v1.0.0

# Custom service user
ansible-playbook install.yml -e service.user=bytefreezer -e service.group=bytefreezer

# Custom paths
ansible-playbook install.yml -e paths.binary_path=/opt/bytefreezer/bin/bytefreezer-piper
```

### Multiple Environments

Create different variable files for different environments:

```
group_vars/
├── all.yml           # Common variables
├── production.yml    # Production overrides
├── staging.yml       # Staging overrides
└── development.yml   # Development overrides
```

### Security Considerations

1. **Change default credentials** in PostgreSQL configuration
2. **Use secure connections** (SSL) for production PostgreSQL
3. **Restrict network access** to monitoring ports
4. **Rotate S3 credentials** regularly
5. **Use Ansible Vault** for sensitive variables

## Support

For issues with:
- **Installation**: Check troubleshooting section above
- **Configuration**: Review `group_vars/all.yml` and service logs
- **PostgreSQL**: Ensure connectivity and credentials are correct
- **Service startup**: Check systemd service logs with `journalctl -u bytefreezer-piper`