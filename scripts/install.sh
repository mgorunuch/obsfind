#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# GitHub repository information
REPO_OWNER="mgorunuch"
REPO_NAME="obsfind"
INSTALL_DIR="/usr/local/bin"

echo -e "${BLUE}ObsFind Installer${NC}"
echo "This script will download and install ObsFind to ${INSTALL_DIR}"
echo ""

# Check if required tools are installed
for cmd in curl grep cut tr; do
    if ! command -v $cmd >/dev/null 2>&1; then
        echo -e "${RED}Error: $cmd is required but not installed.${NC}"
        exit 1
    fi
done

# Check if sudo is available
command -v sudo >/dev/null 2>&1
HAS_SUDO=$?

echo -e "${BLUE}Fetching latest release...${NC}"
# Get latest release info from GitHub API
API_URL="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest"
RELEASE_DATA=$(curl -s $API_URL)

# Check if the API call was successful
if echo "$RELEASE_DATA" | grep -q "API rate limit exceeded" || echo "$RELEASE_DATA" | grep -q "Not Found"; then
    echo -e "${RED}Error: Could not fetch release data from GitHub.${NC}"
    echo "Please check your internet connection or try again later."
    exit 1
fi

# Extract the version number and tag name
VERSION=$(echo "$RELEASE_DATA" | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4 | tr -d 'v')
if [ -z "$VERSION" ]; then
    echo -e "${RED}Error: Could not determine the latest version.${NC}"
    exit 1
fi

echo -e "Installing ObsFind ${GREEN}v${VERSION}${NC}"

# Determine system architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
        echo "ObsFind currently supports x86_64 and arm64 architectures."
        exit 1
        ;;
esac

# Determine operating system
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    darwin)
        OS="darwin"
        ;;
    linux)
        OS="linux"
        ;;
    *)
        echo -e "${RED}Error: Unsupported operating system: $OS${NC}"
        echo "ObsFind currently supports macOS and Linux."
        exit 1
        ;;
esac

# Define the download URLs
CLI_DOWNLOAD_URL="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/v$VERSION/obsfind-$OS-$ARCH"
DAEMON_DOWNLOAD_URL="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/v$VERSION/obsfindd-$OS-$ARCH"

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo -e "${BLUE}Downloading ObsFind CLI...${NC}"
curl -L -s --progress-bar "$CLI_DOWNLOAD_URL" -o "$TMP_DIR/obsfind"

echo -e "${BLUE}Downloading ObsFind Daemon...${NC}"
curl -L -s --progress-bar "$DAEMON_DOWNLOAD_URL" -o "$TMP_DIR/obsfindd"

# Make binaries executable
chmod +x "$TMP_DIR/obsfind" "$TMP_DIR/obsfindd"

# Move binaries to install directory
echo -e "${BLUE}Installing ObsFind to $INSTALL_DIR...${NC}"
if [ -w "$INSTALL_DIR" ]; then
    # We have write permissions, use direct install
    mv "$TMP_DIR/obsfind" "$INSTALL_DIR/obsfind"
    mv "$TMP_DIR/obsfindd" "$INSTALL_DIR/obsfindd"
elif [ $HAS_SUDO -eq 0 ]; then
    # We don't have write permissions but sudo is available
    echo "Elevated permissions needed to write to $INSTALL_DIR"
    sudo mv "$TMP_DIR/obsfind" "$INSTALL_DIR/obsfind"
    sudo mv "$TMP_DIR/obsfindd" "$INSTALL_DIR/obsfindd"
else
    # No write permissions and no sudo
    echo -e "${RED}Error: You don't have permission to write to $INSTALL_DIR${NC}"
    echo "Please run the final installation step manually with appropriate permissions:"
    echo ""
    echo "  sudo mv \"$TMP_DIR/obsfind\" \"$INSTALL_DIR/obsfind\""
    echo "  sudo mv \"$TMP_DIR/obsfindd\" \"$INSTALL_DIR/obsfindd\""
    echo ""
    exit 1
fi

echo -e "${GREEN}✅ ObsFind v${VERSION} installed successfully!${NC}"
echo ""
echo "To start using ObsFind:"
echo ""
echo "1. Initialize the configuration:"
echo "   obsfind config init"
echo ""
echo "2. Start the daemon:"
echo "   obsfindd"
echo ""
echo "3. Search your vault:"
echo "   obsfind search \"your query\""
echo ""
echo "For more information, run:"
echo "   obsfind --help"
echo ""
echo -e "${BLUE}Happy searching!${NC}"