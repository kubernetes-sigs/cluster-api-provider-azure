/*
Copyright 2019 The Kubernetes Authors.

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
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
)

const (
	// TODO: Make config Azure specific
	// TODO: Add cloud provider config back to ClusterConfiguration apiServer once we handle Azure AAD auth (either via creds or MSI)
	/*
		extraArgs:
			cloud-provider: azure
	*/
	// TODO: Add cloud provider config back to InitConfiguration nodeRegistration once we handle Azure AAD auth (either via creds or MSI)
	/*
	  kubeletExtraArgs:
	    cloud-provider: azure
	*/
	controlPlaneBashScript = `{{.Header}}

set -eox

mkdir -p /etc/kubernetes/pki/etcd

echo -n '{{.CloudProviderConfig}}' > /etc/kubernetes/azure.json
echo -n '{{.CACert}}' > /etc/kubernetes/pki/ca.crt
echo -n '{{.CAKey}}' > /etc/kubernetes/pki/ca.key
chmod 600 /etc/kubernetes/pki/ca.key

echo -n '{{.EtcdCACert}}' > /etc/kubernetes/pki/etcd/ca.crt
echo -n '{{.EtcdCAKey}}' > /etc/kubernetes/pki/etcd/ca.key
chmod 600 /etc/kubernetes/pki/etcd/ca.key

echo -n '{{.FrontProxyCACert}}' > /etc/kubernetes/pki/front-proxy-ca.crt
echo -n '{{.FrontProxyCAKey}}' > /etc/kubernetes/pki/front-proxy-ca.key
chmod 600 /etc/kubernetes/pki/front-proxy-ca.key

echo -n '{{.SaCert}}' > /etc/kubernetes/pki/sa.pub
echo -n '{{.SaKey}}' > /etc/kubernetes/pki/sa.key
chmod 600 /etc/kubernetes/pki/sa.key

PRIVATE_IP=$(curl -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2018-10-01&format=text")

cat >/tmp/kubeadm.yaml <<EOF
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: ClusterConfiguration
apiServer:
  certSANs:
    - "$PRIVATE_IP"
    - "{{.LBAddress}}"
    - "{{.InternalLBAddress}}"
  extraArgs:
    cloud-config: /etc/kubernetes/azure.json
    cloud-provider: azure
  extraVolumes:
  - hostPath: /etc/kubernetes/azure.json
    mountPath: /etc/kubernetes/azure.json
    name: cloud-config
    readOnly: true
controllerManager:
  extraArgs:
    cloud-config: /etc/kubernetes/azure.json
    cloud-provider: azure
  extraVolumes:
  - hostPath: /etc/kubernetes/azure.json
    mountPath: /etc/kubernetes/azure.json
    name: cloud-config
    readOnly: true
controlPlaneEndpoint: "{{.LBAddress}}:6443"
clusterName: "{{.ClusterName}}"
networking:
  dnsDomain: "{{.ServiceDomain}}"
  podSubnet: "{{.PodSubnet}}"
  serviceSubnet: "{{.ServiceSubnet}}"
kubernetesVersion: "{{.KubernetesVersion}}"
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: InitConfiguration
nodeRegistration:
  criSocket: /var/run/containerd/containerd.sock
  kubeletExtraArgs:
    cloud-provider: azure
    cloud-config: /etc/kubernetes/azure.json
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
wget --tries 10 https://storage.googleapis.com/cri-containerd-release/cri-containerd-${CONTAINERD_VERSION}.linux-amd64.tar.gz

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

kubeadm init --config /tmp/kubeadm.yaml --v 10 || true
`

	controlPlaneJoinBashScript = `{{.Header}}
    
set -eox

mkdir -p /etc/kubernetes/pki/etcd

echo -n '{{.CloudProviderConfig}}' > /etc/kubernetes/azure.json
echo -n '{{.CACert}}' > /etc/kubernetes/pki/ca.crt
echo -n '{{.CAKey}}' > /etc/kubernetes/pki/ca.key
chmod 600 /etc/kubernetes/pki/ca.key

echo -n '{{.EtcdCACert}}' > /etc/kubernetes/pki/etcd/ca.crt
echo -n '{{.EtcdCAKey}}' > /etc/kubernetes/pki/etcd/ca.key
chmod 600 /etc/kubernetes/pki/etcd/ca.key

echo -n '{{.FrontProxyCACert}}' > /etc/kubernetes/pki/front-proxy-ca.crt
echo -n '{{.FrontProxyCAKey}}' > /etc/kubernetes/pki/front-proxy-ca.key
chmod 600 /etc/kubernetes/pki/front-proxy-ca.key

echo -n '{{.SaCert}}' > /etc/kubernetes/pki/sa.pub
echo -n '{{.SaKey}}' > /etc/kubernetes/pki/sa.key
chmod 600 /etc/kubernetes/pki/sa.key

PRIVATE_IP=$(curl -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2018-10-01&format=text")

cat >/tmp/kubeadm-controlplane-join-config.yaml <<EOF
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
  criSocket: /var/run/containerd/containerd.sock
  kubeletExtraArgs:
    cloud-provider: azure
    cloud-config: /etc/kubernetes/azure.json
controlPlane:
  localAPIEndpoint:
    advertiseAddress: "${PRIVATE_IP}"
    bindPort: 6443
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

kubeadm join --config /tmp/kubeadm-controlplane-join-config.yaml --v 10 || true
`
)

func isKeyPairValid(cert, key string) bool {
	return cert != "" && key != ""
}

// ControlPlaneInput defines the context to generate a control plane instance user data.
type ControlPlaneInput struct {
	baseConfig

	CACert              string
	CAKey               string
	EtcdCACert          string
	EtcdCAKey           string
	FrontProxyCACert    string
	FrontProxyCAKey     string
	SaCert              string
	SaKey               string
	LBAddress           string
	InternalLBAddress   string
	ClusterName         string
	PodSubnet           string
	ServiceDomain       string
	ServiceSubnet       string
	KubernetesVersion   string
	CloudProviderConfig string
	ContainerdVersion   string
	ContainerdSHA256    string
}

// ControlPlaneJoinInput defines context to generate control plane instance user data for control plane node join.
type ControlPlaneJoinInput struct {
	baseConfig

	CACertHash          string
	CACert              string
	CAKey               string
	EtcdCACert          string
	EtcdCAKey           string
	FrontProxyCACert    string
	FrontProxyCAKey     string
	SaCert              string
	SaKey               string
	BootstrapToken      string
	LBAddress           string
	KubernetesVersion   string
	CloudProviderConfig string
	ContainerdVersion   string
	ContainerdSHA256    string
}

func (cpi *ControlPlaneInput) validateCertificates() error {
	if !isKeyPairValid(cpi.CACert, cpi.CAKey) {
		return errors.New("CA cert material in the ControlPlaneInput is missing cert/key")
	}

	if !isKeyPairValid(cpi.EtcdCACert, cpi.EtcdCAKey) {
		return errors.New("etcd CA cert material in the ControlPlaneInput is missing cert/key")
	}

	if !isKeyPairValid(cpi.FrontProxyCACert, cpi.FrontProxyCAKey) {
		return errors.New("Front proxy CA cert material in ControlPlaneInput is missing cert/key")
	}

	if !isKeyPairValid(cpi.SaCert, cpi.SaKey) {
		return errors.New("Service account cert material in ControlPlaneInput is missing cert/key")
	}

	return nil
}

func (cpi *ControlPlaneJoinInput) validateCertificates() error {
	if !isKeyPairValid(cpi.CACert, cpi.CAKey) {
		return errors.New("CA cert material in the ControlPlaneInput is missing cert/key")
	}

	if !isKeyPairValid(cpi.EtcdCACert, cpi.EtcdCAKey) {
		return errors.New("etcd CA cert material in the ControlPlaneInput is missing cert/key")
	}

	if !isKeyPairValid(cpi.FrontProxyCACert, cpi.FrontProxyCAKey) {
		return errors.New("Front proxy CA cert material in ControlPlaneInput is missing cert/key")
	}

	if !isKeyPairValid(cpi.SaCert, cpi.SaKey) {
		return errors.New("Service account cert material in ControlPlaneInput is missing cert/key")
	}

	return nil
}

// NewControlPlane returns the user data string to be used on a control plane instance.
func NewControlPlane(input *ControlPlaneInput) (string, error) {
	input.Header = defaultHeader
	if err := input.validateCertificates(); err != nil {
		return "", errors.Wrapf(err, "ControlPlaneInput is invalid")
	}

	config, err := generate(infrav1.ControlPlane, controlPlaneBashScript, input)
	if err != nil {
		return "", errors.Wrapf(err, "failed to generate user data for new control plane machine")
	}

	return config, err
}

// JoinControlPlane returns the user data string to be used on a new control plane instance.
func JoinControlPlane(input *ControlPlaneJoinInput) (string, error) {
	input.Header = defaultHeader

	if err := input.validateCertificates(); err != nil {
		return "", errors.Wrapf(err, "ControlPlaneInput is invalid")
	}

	config, err := generate(infrav1.ControlPlane, controlPlaneJoinBashScript, input)
	if err != nil {
		return "", errors.Wrapf(err, "failed to generate user data for machine joining control plane")
	}
	return config, err
}
