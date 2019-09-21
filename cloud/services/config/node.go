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

import (
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
)

const (
	// TODO: Make config Azure specific
	// TODO: Add cloud provider config back to InitConfiguration nodeRegistration once we handle Azure AAD auth (either via creds or MSI)
	/*
	  kubeletExtraArgs:
	    cloud-provider: azure
	*/
	nodeBashScript = `{{.Header}}

mkdir -p /etc/kubernetes/pki

echo -n '{{.CloudProviderConfig}}' > /etc/kubernetes/azure.json

cat >/tmp/kubeadm-node.yaml <<EOF
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: JoinConfiguration
discovery:
  bootstrapToken:
    token: "{{.BootstrapToken}}"
    apiServerEndpoint: "{{.InternalLBAddress}}:6443"
    caCertHashes:
      - "{{.CACertHash}}"
nodeRegistration:
  criSocket: /var/run/containerd/containerd.sock
  kubeletExtraArgs:
    cloud-provider: azure
    cloud-config: /etc/kubernetes/azure.json
    node-labels: "node-role.kubernetes.io/node="
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
export CONTAINERD_VERSION="{{.ContainerdVersion}}"
export CONTAINERD_SHA256="{{.ContainerdSHA256}}"

# Download containerd tar.
curl --connect-timeout 5 --retry 20 --retry-delay 0 --retry-max-time 120 -O https://storage.googleapis.com/cri-containerd-release/cri-containerd-${CONTAINERD_VERSION}.linux-amd64.tar.gz

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
apt-get install -y kubernetes-cni=0.6.0-00
apt-get install -y kubelet="{{.KubernetesVersion}}-00" kubeadm="{{.KubernetesVersion}}-00" kubectl="{{.KubernetesVersion}}-00"
apt-mark hold kubelet kubeadm kubectl

kubeadm join --config /tmp/kubeadm-node.yaml || true
`
)

// NodeInput defines the context to generate a node user data.
type NodeInput struct {
	baseConfig

	CACertHash          string
	BootstrapToken      string
	InternalLBAddress   string
	KubernetesVersion   string
	CloudProviderConfig string
	ContainerdVersion   string
	ContainerdSHA256    string
}

// NewNode returns the user data string to be used on a node instance.
func NewNode(input *NodeInput) (string, error) {
	input.Header = defaultHeader
	return generate(infrav1.Node, nodeBashScript, input)
}
