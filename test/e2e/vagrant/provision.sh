#!/bin/bash
set -e

echo "📦 Provisioning Test VM..."

# Force DNS resolution (aggressive fix)
sudo rm -f /etc/resolv.conf
echo "nameserver 8.8.8.8" | sudo tee /etc/resolv.conf
echo "nameserver 1.1.1.1" | sudo tee -a /etc/resolv.conf

# Update and install basic tools
sudo apt-get update
sudo apt-get install -y git curl

# Install Go 1.24 (Manual because apt is old)
if [ ! -d "/usr/local/go" ]; then
    echo "📦 Installing Go 1.24..."
    curl -L https://go.dev/dl/go1.24.0.linux-amd64.tar.gz -o go.tar.gz
    sudo tar -C /usr/local -xzf go.tar.gz
    rm go.tar.gz
fi

# Set PATH for vagrant user permanently
echo 'export PATH=$PATH:/usr/local/go/bin' >> /home/vagrant/.bashrc
echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc

echo "✅ Provisioning Complete"
