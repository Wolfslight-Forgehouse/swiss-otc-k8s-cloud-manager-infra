#!/bin/bash
# Swiss OTC Jumpserver initialization script

set -e

echo "🚀 Initializing Swiss OTC Jumpserver..."

# Update system
apt-get update
apt-get upgrade -y

# Install essential tools
apt-get install -y \
    curl \
    wget \
    git \
    vim \
    htop \
    jq \
    unzip \
    software-properties-common \
    apt-transport-https \
    ca-certificates \
    gnupg \
    lsb-release

# Install Docker
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io
usermod -aG docker ubuntu

# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install helm
curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | tee /usr/share/keyrings/helm.gpg > /dev/null
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | tee /etc/apt/sources.list.d/helm-stable-debian.list
apt-get update
apt-get install -y helm

# Install terraform
curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor | tee /usr/share/keyrings/hashicorp.gpg > /dev/null
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/hashicorp.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/hashicorp.list
apt-get update
apt-get install -y terraform

# Install terragrunt
TERRAGRUNT_VERSION=$(curl -s https://api.github.com/repos/gruntwork-io/terragrunt/releases/latest | jq -r .tag_name)
wget -O /usr/local/bin/terragrunt "https://github.com/gruntwork-io/terragrunt/releases/download/$TERRAGRUNT_VERSION/terragrunt_linux_amd64"
chmod +x /usr/local/bin/terragrunt

# Setup SSH key
if [ ! -z "${ssh_public_key}" ]; then
    mkdir -p /home/ubuntu/.ssh
    echo "${ssh_public_key}" >> /home/ubuntu/.ssh/authorized_keys
    chown -R ubuntu:ubuntu /home/ubuntu/.ssh
    chmod 700 /home/ubuntu/.ssh
    chmod 600 /home/ubuntu/.ssh/authorized_keys
fi

# Create Swiss OTC tools directory
mkdir -p /opt/swiss-otc
cd /opt/swiss-otc

# Clone Swiss OTC cloud manager (our implementation)
# git clone https://github.com/your-org/swiss-otc-cloud-manager.git || echo "Repository not yet public"

# Create kubectl config directory
mkdir -p /home/ubuntu/.kube
chown ubuntu:ubuntu /home/ubuntu/.kube

# Create cluster access script
cat > /usr/local/bin/cluster-connect << 'SCRIPT'
#!/bin/bash
# Connect to RKE2 cluster from jumpserver

CLUSTER_NAME="${cluster_name}"
MASTER_IP="$1"

if [ -z "$MASTER_IP" ]; then
    echo "Usage: cluster-connect <master-private-ip>"
    echo "Example: cluster-connect 10.0.1.10"
    exit 1
fi

echo "🔗 Connecting to RKE2 cluster: $CLUSTER_NAME"
echo "   Master node: $MASTER_IP"

# Copy kubeconfig from master node
echo "📋 Copying kubeconfig..."
scp -o StrictHostKeyChecking=no ubuntu@$MASTER_IP:/etc/rancher/rke2/rke2.yaml ~/.kube/config

# Update server URL to use private IP
sed -i "s/127.0.0.1:6443/$MASTER_IP:6443/" ~/.kube/config

echo "✅ Cluster connection configured!"
echo "🧪 Testing connection..."

if kubectl cluster-info; then
    echo "🎉 Successfully connected to RKE2 cluster!"
    echo ""
    echo "🔧 Available commands:"
    echo "   kubectl get nodes"
    echo "   kubectl get pods -A"
    echo "   helm list -A"
else
    echo "❌ Connection failed - check master IP and SSH access"
fi
SCRIPT

chmod +x /usr/local/bin/cluster-connect

# Create Swiss OTC cloud controller deployment script  
cat > /usr/local/bin/deploy-cloud-controller << 'SCRIPT'
#!/bin/bash
# Deploy Swiss OTC cloud controller to RKE2 cluster

echo "🚀 Deploying Swiss OTC Cloud Controller..."

if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "❌ Not connected to cluster. Run: cluster-connect <master-ip>"
    exit 1
fi

# Deploy cloud controller
kubectl apply -f /opt/swiss-otc/swiss-otc-cloud-manager/examples/kubernetes/

echo "✅ Swiss OTC Cloud Controller deployed!"
echo ""
echo "🧪 Test LoadBalancer creation:"
echo "   kubectl create deployment nginx --image=nginx"
echo "   kubectl expose deployment nginx --type=LoadBalancer --port=80"
echo "   kubectl get services # Should show Swiss OTC EIP!"
SCRIPT

chmod +x /usr/local/bin/deploy-cloud-controller

echo "✅ Swiss OTC Jumpserver initialization complete!"
echo ""
echo "🎯 Available tools:"
echo "   - kubectl (Kubernetes CLI)"
echo "   - helm (Package manager)"  
echo "   - terraform + terragrunt (Infrastructure)"
echo "   - docker (Container runtime)"
echo "   - git, vim, htop, jq"
echo ""
echo "🔧 Swiss OTC commands:"
echo "   - cluster-connect <master-ip>"
echo "   - deploy-cloud-controller"
echo ""
echo "📋 Next steps:"
echo "   1. SSH to jumpserver: ssh ubuntu@<jumpserver-eip>"
echo "   2. Connect to cluster: cluster-connect <master-private-ip>"
echo "   3. Deploy cloud controller: deploy-cloud-controller"
echo "   4. Test LoadBalancer: kubectl expose deployment <name> --type=LoadBalancer"
