/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

const (
	// TODO: Make config Azure specific
	nodeBashScript = `{{.Header}}

HOSTNAME="$(curl -H Metadata:true "http://169.254.169.254/metadata/instance/compute/name?api-version=2018-10-01&format=text")"

cat >/tmp/kubeadm-node.yaml <<EOF
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: JoinConfiguration
discovery:
  bootstrapToken:
    token: "{{.BootstrapToken}}"
    apiServerEndpoint: "{{.LBAddress}}:6443"
    caCertHashes:
      - "{{.CACertHash}}"
nodeRegistration:
  name: "${HOSTNAME}"
  criSocket: /var/run/containerd/containerd.sock
  kubeletExtraArgs:
    # TODO: Re-enable once we handle Azure AAD auth (either via creds or MSI)
    #cloud-provider: azure
EOF

# Configure containerd prerequisites
modprobe overlay
modprobe br_netfilter

# Setup required sysctl params, these persist across reboots.
cat > /etc/sysctl.d/99-kubernetes-cri.conf <<EOF
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

sysctl --system

apt-get install -y libseccomp2

# Install containerd
# Export required environment variables.
export CONTAINERD_VERSION="1.2.4"
export CONTAINERD_SHA256="3391758c62d17a56807ddac98b05487d9e78e5beb614a0602caab747b0eda9e0"

# Download containerd tar.
wget https://storage.googleapis.com/cri-containerd-release/cri-containerd-${CONTAINERD_VERSION}.linux-amd64.tar.gz

# Check hash.
echo "${CONTAINERD_SHA256} cri-containerd-${CONTAINERD_VERSION}.linux-amd64.tar.gz" | sha256sum --check -

# Unpack.
tar --no-overwrite-dir -C / -xzf cri-containerd-${CONTAINERD_VERSION}.linux-amd64.tar.gz

# Start containerd.
systemctl start containerd

# Install kubeadm (https://kubernetes.io/docs/setup/independent/install-kubeadm/)
apt-get update && apt-get install -y apt-transport-https curl
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb https://apt.kubernetes.io/ kubernetes-xenial main
EOF
apt-get update
apt-get install -y kubelet kubeadm kubectl
apt-mark hold kubelet kubeadm kubectl

kubeadm join --config /tmp/kubeadm-node.yaml
`
)

// NodeInput defines the context to generate a node user data.
type NodeInput struct {
	baseConfig

	CACertHash     string
	BootstrapToken string
	LBAddress      string
}

// NewNode returns the user data string to be used on a node instance.
func NewNode(input *NodeInput) (string, error) {
	input.Header = defaultHeader
	return generate("node", nodeBashScript, input)
}
