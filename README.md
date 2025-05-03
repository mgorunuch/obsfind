# ObsFind

Semantic search for Obsidian vaults using vector embeddings.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/mgorunuch/obsfind/main/scripts/install.sh | bash
```

The script will automatically use sudo only if needed for installation to /usr/local/bin.

## Overview

ObsFind provides semantic search capabilities for your Obsidian vault by:
- Creating vector embeddings of your notes
- Storing them in Qdrant (vector database)
- Enabling search through CLI or Obsidian plugin
- Automatically updating index as files change

Want to contribute? Check out our [current tasks](TASKS.md) and [how to contribute](CONTRIBUTING.md).

## Features

- Semantic search based on meaning, not just keywords
- Real-time indexing when files change
- Multiple chunking strategies (header-based, sliding window, hybrid)
- Local embedding generation with Ollama
- Tag and path filtering

## Architecture

```
┌──────────┐     ┌──────────┐    ┌───────────┐
│ Obsidian │     │   CLI    │    │Plugin(WIP)│
│  Vault   │     │ Interface│    │ Interface │
└────┬─────┘     └────┬─────┘    └─────┬─────┘
     │                │                │
     ▼                ▼                ▼
┌───────────────────────────────────────────┐
│            Background Daemon              │
│                                           │
│ ┌───────────┐   ┌───────────┐  ┌────────┐ │
│ │   File    │   │ Indexing  │  │ Query  │ │
│ │  Watcher  │◄─►│ Service   │◄─┤ Handler│ │
│ └───────────┘   └────┬──────┘  └────┬───┘ │
└──────────────────────┼──────────────┼─────┘
                       │              │
                       ▼              ▼
                 ┌──────────┐    ┌──────────┐
                 │  Qdrant  │    │  Ollama  │
                 │ Database │    │ Embedder │
                 └──────────┘    └──────────┘
```

## Usage

### Start the daemon
```bash
obsfindd
```

### Search your vault
```bash
obsfind search "machine learning concepts"
obsfind search --tags work,important "project deadlines"
obsfind search --limit 15 --score 0.7 "climate change solutions"
```

### Find similar documents
```bash
obsfind similar path/to/document.md
```

### Check daemon status
```bash
obsfind status
```

### Reindex your vault
```bash
obsfind reindex
```

### Manage vault paths
```bash
# List configured vault paths
obsfind vault list

# Add a new vault path
obsfind vault add ~/Documents/NewVault

# Remove a vault path
obsfind vault remove ~/Documents/OldVault
```

## Configuration

Configuration file is located at `~/.config/obsfind/config.yaml`.

### Configuration Commands

```bash
# Create a new default configuration
obsfind config init

# View current configuration
obsfind config view

# Show config file path
obsfind config path

# Generate a configuration template
obsfind config template
```

Example configuration:
```yaml
general:
  debug: false
  data_dir: ~/.obsfind

paths:
  vault_path: ~/Documents/Obsidian/MyVault

embedding:
  provider: ollama
  model: nomic-embed-text
  server_url: http://localhost:11434

qdrant:
  host: localhost
  grpc_port: 6334
  collection_name: obsfind

indexing:
  chunking_strategy: hybrid
  include_patterns:
    - "*.md"
  exclude_patterns:
    - ".obsidian/*"
    - ".git/*"

api:
  host: localhost
  port: 8080
```

## Technical Details

- Uses Ollama with nomic-embed-text model for local embedding
- Qdrant for vector storage and search
- Written in Go for performance and concurrency
- Multiple chunking strategies for better semantic understanding

## Building and Installation

Use the provided script to build and install ObsFind:

```bash
# Build and install ObsFind
./scripts/build_install.sh
```

This script will:
- Build both the daemon and CLI components
- Install binaries to the correct location
- Set up autostart services (optional)

For system autostart configuration, the script will detect your platform and set up the appropriate service.

## Contributing

We welcome contributions to ObsFind! Here's how you can help:

### Getting Started

1. Review open [tasks and issues](TASKS.md)
2. Set up your development environment
3. Make your changes on a feature branch
4. Submit a pull request

### Development Setup

```bash
# Clone the repository
git clone https://github.com/yourusername/obsfind.git
cd obsfind

# Install dependencies
go mod download

# Run tests
go test -v ./...

# Build the project
./scripts/build_install.sh
```

### Contribution Areas

- **Documentation**: Help improve user guides and code documentation
- **Testing**: Add unit and integration tests
- **Cross-Platform Support**: Test and fix issues on different platforms
- **Performance Optimization**: Help improve indexing and search speed
- **Obsidian Plugin**: Contribute to the Obsidian plugin development

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines and [TASKS.md](TASKS.md) for current project tasks.
