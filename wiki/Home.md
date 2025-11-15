# OxiCleanarr Wiki

Welcome to the OxiCleanarr wiki! OxiCleanarr is a lightweight media cleanup automation tool for the *arr stack with Jellyfin integration.

> **"But wait, there's more!"** - Built with the power and effectiveness you'd expect from a product endorsed by Billy Mays himself!

## Quick Navigation

### Getting Started
- [Installation Guide](Installation-Guide) - Docker and build-from-source instructions
- [Quick Start](Quick-Start) - Get up and running in minutes
- [Configuration](Configuration) - Complete configuration reference

### Features
- [Deletion Timeline](Deletion-Timeline) - Visual timeline of scheduled deletions
- [Leaving Soon Library](Leaving-Soon-Library) - Create preview libraries in Jellyfin
- [Advanced Rules](Advanced-Rules) - Tag-based, user-based, and watched-based cleanup

### Deployment
- [Docker Deployment](Docker-Deployment) - Container-based deployment
- [NAS Deployment](NAS-Deployment) - Synology/QNAP specific guide
- [Environment Variables](Environment-Variables) - Configuration via environment

### API & Development
- [REST API Reference](API-Reference) - Complete API documentation
- [Development Guide](Development-Guide) - Contributing and building
- [Architecture](Architecture) - System design and components

### Integrations
- [Jellyfin Integration](Jellyfin-Integration) - Media server setup
- [Radarr/Sonarr Integration](Radarr-Sonarr-Integration) - Media management
- [Jellyseerr Integration](Jellyseerr-Integration) - Request tracking (optional)
- [Jellystat Integration](Jellystat-Integration) - Watch history tracking (optional)

### Help & Troubleshooting
- [Troubleshooting](Troubleshooting) - Common issues and solutions
- [FAQ](FAQ) - Frequently asked questions
- [Security](Security) - Security considerations

## Key Features

- 🪶 **Lightweight** - 15MB Docker image, <40MB RAM usage
- ⚡ **Fast** - <50ms startup, <100ms API responses
- 🎯 **Simple Config** - Sensible defaults, minimal YAML
- 👀 **Deletion Visibility** - Timeline view, countdown timers, "Keep" button
- 🔄 **Hot-Reload** - Live config changes without restart
- 🎨 **Modern UI** - React 19 + shadcn/ui

## Screenshots

### Dashboard
![Dashboard](../docs/screenshots/dashboard.png)
*Overview of your media cleanup status with key metrics*

### Timeline View
![Timeline](../docs/screenshots/timeline.png)
*Visual timeline showing when media items are scheduled for deletion*

### Library Management
![Library](../docs/screenshots/library.png)
*Browse and manage your entire media library*

## Project Links

- **GitHub Repository**: [ramonskie/oxicleanarr](https://github.com/ramonskie/oxicleanarr)
- **Docker Images**: [ghcr.io/ramonskie/oxicleanarr](https://github.com/ramonskie/oxicleanarr/pkgs/container/oxicleanarr)
- **Bridge Plugin**: [jellyfin-plugin-oxicleanarr](https://github.com/ramonskie/jellyfin-plugin-oxicleanarr)

## Community & Support

- Report issues on [GitHub Issues](https://github.com/ramonskie/oxicleanarr/issues)
- Check existing issues before creating new ones
- Include logs and configuration when reporting bugs

## About

**Built with AI**: This project was created ~90% with AI assistance using [OpenCode](https://opencode.ai/) and Claude 4.5 Sonnet.

**License**: MIT License
