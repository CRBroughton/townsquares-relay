# Townsquares relay

### Townsquares - Decentralised community focused social networks

This repository is home to the WIP Townsquares server/relay. The focus of this repository
is to built a decentralised, mesh-based relay system for hyper-localised communities.

#### Technical Goals

- ~~Create basic implementation of the relay server~~ ✅
- ~~Ensure the system is resiliant and propogates events relay-to-relay~~ ✅
- Ensure events are de-duped across the network when shared relay-to-relay
- Ensure users can use existing keys, ideally with existing 'signer' applications
- Ensure mesh 'bridges' don't propogate relay events to no-direct relays (avoid event leakage)
- ~~Create a dockerised implementation of the relay~~ ✅
- Explore other deployment options (desktop application, flatpak, TUI etc)


#### Documentation Goals

- Create basic README.md file for instructions on how to get up and running
- Create a Mermaid graph showcasing how clients and relays connect to each other
- Create a guide on how to set-up and deploy a Townsquares relay

## Tailscale Integration

Townsquares relay now supports Tailscale integration for easy networking between community relays without complex proxy configurations.

### Setup

1. **Install Tailscale** on your server (if not using tsnet auth key):
   ```bash
   curl -fsSL https://tailscale.com/install.sh | sh
   tailscale up
   ```

2. **Generate an auth key** (optional, but recommended for automated deployment):
   - Go to [Tailscale Admin Console](https://login.tailscale.com/admin/settings/keys)
   - Generate a new auth key (consider using a reusable key for multiple relays)

3. **Configure your relay** with Tailscale settings:
   ```json
   {
     "port": ":4443",
     "name": "Community Relay 1",
     "pubkey": "YOUR_ACTUAL_PUB_KEY_HERE",
     "description": "A townsquares relay for our community",
     "relays": ["http://community-relay-2:4443", "http://community-relay-3:4443"],
     "db_path": "db",
     "tailscale_enabled": true,
     "tailscale_auth_key": "tskey-auth-xxxxxxxxxxxx", //optional
     "tailscale_hostname": "my-townsquare-relay",
     "tailscale_https": false,
     "tailscale_state_dir": "tailscale-relay-state"
   }
   ```

4. **Start the relay**:
   ```bash
   ./townsquares-relay config.json
   ```

### Configuration Options

- `tailscale_enabled`: Enable/disable Tailscale integration
- `tailscale_auth_key`: Optional auth key for automatic device approval (optional)
- `tailscale_hostname`: Hostname for this device on the tailnet
- `tailscale_https`: Use HTTPS with automatic TLS certificates (requires port 443)
- `tailscale_state_dir`: Directory for storing Tailscale state (optional)

### Inter-Relay Communication

When Tailscale is enabled, relays can connect to each other using Tailscale hostnames:

```json
{
  "relays": [
    "http://community-relay-2",
    "https://community-relay-3:443"
  ]
}
```

The relay manager will automatically use the Tailscale HTTP client for these connections, enabling secure communication within the tailnet.