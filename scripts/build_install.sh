#!/bin/bash
# Build and install ObsFind binaries to $PATH

set -e

# Get project directory
PROJECT_DIR=$(cd "$(dirname "$(dirname "$0")")" && pwd)
echo "Project directory: $PROJECT_DIR"

# Build directory
BUILD_DIR="$PROJECT_DIR/bin"
mkdir -p "$BUILD_DIR"

# Version from git or fallback to 0.1.0
VERSION=$(git describe --tags 2>/dev/null || echo "0.1.0")

# Determine destination directory
if [ -n "$1" ]; then
    INSTALL_DIR="$1"
elif [ -d "$HOME/go/bin" ]; then
    INSTALL_DIR="$HOME/go/bin"
elif [ -d "$GOPATH/bin" ]; then
    INSTALL_DIR="$GOPATH/bin"
else
    INSTALL_DIR="/usr/local/bin"
    echo "Warning: Installing to $INSTALL_DIR requires sudo"
fi

echo "Building ObsFind binaries (version: $VERSION)..."

# Build CLI
echo "Building CLI..."
cd "$PROJECT_DIR"
go build -ldflags "-X main.version=$VERSION" -o "$BUILD_DIR/obsfind" ./src/cmd/cli
echo "✓ CLI built successfully"

# Build daemon
echo "Building daemon..."
cd "$PROJECT_DIR"
go build -ldflags "-X main.version=$VERSION" -o "$BUILD_DIR/obsfindd" ./src/cmd/daemon
echo "✓ Daemon built successfully"

# Code sign the binaries on macOS
echo "Signing binaries with ad-hoc signature for macOS..."
codesign --force --options runtime --sign - "$BUILD_DIR/obsfind"
codesign --force --options runtime --sign - "$BUILD_DIR/obsfindd"
echo "✓ Binaries signed successfully"

# Create configuration directory
CONFIG_DIR="$HOME/.config/obsfind"
mkdir -p "$CONFIG_DIR"

# Copy sample config if it doesn't exist
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    echo "Creating default configuration file..."
    cp "$PROJECT_DIR/docker/config.yaml" "$CONFIG_DIR/config.yaml"
    
    # Update paths in the config
    sed -i.bak "s|/vault|$HOME/Documents/Obsidian|g" "$CONFIG_DIR/config.yaml"
    rm -f "$CONFIG_DIR/config.yaml.bak"
    
    echo "✓ Default configuration created at $CONFIG_DIR/config.yaml"
    echo "  Please edit this file to match your Obsidian vault location"
fi

# Ask for installation
echo ""
echo "Ready to install ObsFind to $INSTALL_DIR"
read -p "Continue with installation? [Y/n] " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
    # Install binaries
    # Check if we need sudo (system directories)
    if [[ "$INSTALL_DIR" == "/bin" || "$INSTALL_DIR" == "/usr/bin" || "$INSTALL_DIR" == "/usr/local/bin" ]]; then
        echo "Installing with sudo to $INSTALL_DIR..."
        sudo mkdir -p "$INSTALL_DIR"
        sudo cp "$BUILD_DIR/obsfind" "$INSTALL_DIR/obsfind"
        sudo cp "$BUILD_DIR/obsfindd" "$INSTALL_DIR/obsfindd"
        sudo chmod +x "$INSTALL_DIR/obsfind" "$INSTALL_DIR/obsfindd"
    else
        echo "Installing to $INSTALL_DIR..."
        mkdir -p "$INSTALL_DIR"
        cp "$BUILD_DIR/obsfind" "$INSTALL_DIR/obsfind"
        cp "$BUILD_DIR/obsfindd" "$INSTALL_DIR/obsfindd"
        chmod +x "$INSTALL_DIR/obsfind" "$INSTALL_DIR/obsfindd"
    fi
    
    echo "✅ Installation complete!"
    echo ""
    echo "To start the daemon:"
    echo "  $ obsfindd"
    echo ""
    echo "To use the CLI:"
    echo "  $ obsfind search \"your query\""
    echo ""
    echo "To view help:"
    echo "  $ obsfind --help"
else
    echo "Installation skipped."
fi
