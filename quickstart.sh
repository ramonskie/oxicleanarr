#!/bin/bash
set -e

echo "🚀 Prunarr Quick Start"
echo "====================="
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or higher."
    echo "   Visit: https://golang.org/doc/install"
    exit 1
fi

echo "✓ Go is installed: $(go version)"
echo ""

# Check if config exists
if [ ! -f "./config/prunarr.yaml" ]; then
    echo "📝 Creating configuration file..."
    mkdir -p config
    cp config/prunarr.yaml.example config/prunarr.yaml
    echo "✓ Configuration file created at ./config/prunarr.yaml"
    echo ""
    echo "⚠️  IMPORTANT: Edit ./config/prunarr.yaml with your service URLs and API keys"
    echo ""
    read -p "Press Enter to continue once you've updated the config..."
else
    echo "✓ Configuration file already exists"
fi

echo ""
echo "🔨 Building Prunarr..."
make build

echo ""
echo "🎉 Setup complete!"
echo ""
echo "To start Prunarr:"
echo "  ./prunarr"
echo ""
echo "Or use:"
echo "  make run    - Build and run"
echo "  make dev    - Run in development mode"
echo ""
echo "API endpoints:"
echo "  Health:  http://localhost:8080/health"
echo "  Login:   http://localhost:8080/api/auth/login"
echo ""
echo "Default credentials:"
echo "  Username: admin"
echo "  Password: changeme"
echo ""
