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

package certificates

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcertutil "k8s.io/client-go/util/cert"
	bootstraputil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/klog"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	tokenphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/node"
	certsphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/certs"
	kubeconfigphase "k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pubkeypin"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
)

// Get should implement returning certs and kubeconfigs.
func (s *Service) Get(ctx context.Context, spec v1alpha1.ResourceSpec) (interface{}, error) {
	return nil, errors.New("Not implemented")
}

// Delete cleans up and generated certificates, could be useful for renewal.
func (s *Service) Delete(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	return nil
}

// Reconcile Helper function so this can be unittested.
func (s *Service) Reconcile(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	klog.V(2).Infof("generating certificates")
	clusterName := s.scope.Cluster.Name
	tmpDirName := "/tmp/cluster-api/" + clusterName

	defer os.RemoveAll(tmpDirName)

	v1beta1cfg := &kubeadmv1beta1.InitConfiguration{}
	kubeadmscheme.Scheme.Default(v1beta1cfg)
	v1beta1cfg.CertificatesDir = tmpDirName + "/certs"
	v1beta1cfg.Etcd.Local = &kubeadmv1beta1.LocalEtcd{}
	// 10.0.0.1 is fake api server address, since this is also generated on control plane
	v1beta1cfg.LocalAPIEndpoint = kubeadmv1beta1.APIEndpoint{AdvertiseAddress: "10.0.0.1", BindPort: 6443}
	v1beta1cfg.ControlPlaneEndpoint = fmt.Sprintf("%s:6443", s.scope.Network().APIServerIP.DNSName)
	v1beta1cfg.APIServer.CertSANs = []string{azure.DefaultInternalLBIPAddress}
	// require a fake node name for now, this will be regenerated when it runs on node anyways
	v1beta1cfg.NodeRegistration.Name = "fakenode" + clusterName
	cfg := &kubeadmapi.InitConfiguration{}
	kubeadmscheme.Scheme.Default(cfg)
	kubeadmscheme.Scheme.Convert(v1beta1cfg, cfg, nil)

	if err := CreatePKICertificates(cfg); err != nil {
		return errors.Wrapf(err, "Failed to generate pki certs: %q", err)
	}

	if err := CreateSACertificates(cfg); err != nil {
		return errors.Wrapf(err, "Failed to generate sa certs: %q", err)
	}

	if err := CreateBastionSSHKeys(s.scope.ClusterConfig); err != nil {
		return errors.Wrap(err, "Failed to generate ssh keys for bastion host")
	}

	kubeConfigDir := tmpDirName + "/kubeconfigs"
	if err := CreateKubeconfigs(cfg, kubeConfigDir); err != nil {
		return errors.Wrapf(err, "Failed to generate kubeconfigs: %q", err)
	}

	if err := updateClusterConfigKeyPairs(s.scope.ClusterConfig, tmpDirName); err != nil {
		return errors.Wrapf(err, "Failed to update certificates: %q", err)
	}

	if err := updateClusterConfigKubeConfig(s.scope.ClusterConfig, tmpDirName); err != nil {
		return errors.Wrapf(err, "Failed to update kubeconfigs and discoveryhashes: %q", err)
	}

	klog.V(2).Infof("successfully created certificates")

	return nil
}

// CreatePKICertificates creates base pki assets in cfg.CertDir directory.
func CreatePKICertificates(cfg *kubeadmapi.InitConfiguration) error {
	klog.V(2).Infof("CreatePKIAssets")
	if err := certsphase.CreatePKIAssets(cfg); err != nil {
		return err
	}
	klog.V(2).Infof("CreatePKIAssets success")
	return nil
}

// CreateSACertificates creates sa certificates in cfg.CertDir directory.
func CreateSACertificates(cfg *kubeadmapi.InitConfiguration) error {
	klog.V(2).Infof("CreateSACertificates")
	if err := certsphase.CreateServiceAccountKeyAndPublicKeyFiles(cfg); err != nil {
		return err
	}
	klog.V(2).Infof("CreateSACertificates success")
	return nil
}

// GetDiscoveryHashes returns discovery hashes from a given kubeconfig file.
func GetDiscoveryHashes(kubeConfigFile string) ([]string, error) {
	klog.V(2).Infof("GetDiscoveryHashes")
	// load the kubeconfig file to get the CA certificate and endpoint
	config, err := clientcmd.LoadFromFile(kubeConfigFile)
	if err != nil {
		return nil, err
	}

	// load the default cluster config
	clusterConfig := kubeconfigutil.GetClusterFromKubeConfig(config)
	if clusterConfig == nil {
		return nil, errors.New("failed to get default cluster config")
	}

	// load CA certificates from the kubeconfig (either from PEM data or by file path)
	var caCerts []*x509.Certificate
	if clusterConfig.CertificateAuthorityData != nil {
		caCerts, err = clientcertutil.ParseCertsPEM(clusterConfig.CertificateAuthorityData)
		if err != nil {
			return nil, err
		}
	} else if clusterConfig.CertificateAuthority != "" {
		caCerts, err = clientcertutil.CertsFromFile(clusterConfig.CertificateAuthority)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("no CA certificates found in kubeconfig")
	}

	// hash all the CA certs and include their public key pins as trusted values
	publicKeyPins := make([]string, 0, len(caCerts))
	for _, caCert := range caCerts {
		publicKeyPins = append(publicKeyPins, pubkeypin.Hash(caCert))
	}
	klog.V(2).Infof("GetDiscoveryHashes success")
	return publicKeyPins, nil
}

// CreateNewBootstrapToken creates new bootstrap token using in cluster config.
func CreateNewBootstrapToken(kubeconfig string, tokenTTL time.Duration) (string, error) {
	klog.V(2).Infof("CreateNewBootstrapToken")
	token, err := bootstraputil.GenerateBootstrapToken()
	if err != nil {
		return token, err
	}

	config, err := clientcmd.NewClientConfigFromBytes([]byte(kubeconfig))
	if err != nil {
		return token, err
	}

	cfg, err := config.ClientConfig()
	if err != nil {
		return token, err
	}

	kclientset, err := clientset.NewForConfig(cfg)
	if err != nil {
		return token, err
	}

	tokenString, err := kubeadmapi.NewBootstrapTokenString(token)
	if err != nil {
		return token, err
	}

	bootstrapTokens := []kubeadmapi.BootstrapToken{
		{
			Token:  tokenString,
			TTL:    &metav1.Duration{Duration: tokenTTL},
			Groups: []string{"system:bootstrappers:kubeadm:default-node-token"},
			Usages: []string{"signing", "authentication"},
		},
	}

	if err := tokenphase.CreateNewTokens(kclientset, bootstrapTokens); err != nil {
		return token, err
	}

	klog.V(2).Infof("CreateNewBootstrapToken success %s", token)
	return token, nil
}

// CreateKubeconfigs creates kubeconfigs for all profiles.
func CreateKubeconfigs(cfg *kubeadmapi.InitConfiguration, kubeConfigDir string) error {
	klog.V(2).Infof("CreateKubeconfigs admin kubeconfig")
	if err := kubeconfigphase.CreateKubeConfigFile(kubeadmconstants.AdminKubeConfigFileName, kubeConfigDir, cfg); err != nil {
		return err
	}
	// if err := kubeconfigphase.CreateKubeConfigFile(kubeadmconstants.KubeletKubeConfigFileName, kubeConfigDir, cfg); err != nil {
	// 	return err
	// }
	// if err := kubeconfigphase.CreateKubeConfigFile(kubeadmconstants.ControllerManagerKubeConfigFileName, kubeConfigDir, cfg); err != nil {
	// 	return err
	// }
	// if err := kubeconfigphase.CreateKubeConfigFile(kubeadmconstants.SchedulerKubeConfigFileName, kubeConfigDir, cfg); err != nil {
	// 	return err
	// }
	klog.V(2).Infof("CreateKubeconfigs admin kubeconfig success")
	return nil
}

// updateClusterConfigKeyPairs populates clusterConfig with all the requisite certs.
func updateClusterConfigKeyPairs(clusterConfig *v1alpha1.AzureClusterProviderSpec, tmpDirName string) error {
	certsDir := tmpDirName + "/certs"

	if err := updateCertKeyPair(&clusterConfig.CAKeyPair, certsDir+"/ca"); err != nil {
		return err
	}

	if err := updateCertKeyPair(&clusterConfig.FrontProxyCAKeyPair, certsDir+"/front-proxy-ca"); err != nil {
		return err
	}

	if err := updateCertKeyPair(&clusterConfig.EtcdCAKeyPair, certsDir+"/etcd/ca"); err != nil {
		return err
	}

	if len(clusterConfig.SAKeyPair.Key) <= 0 {
		buf, err := ioutil.ReadFile(certsDir + "/sa.key")
		if err != nil {
			return err
		}
		clusterConfig.SAKeyPair.Key = buf
	}
	if len(clusterConfig.SAKeyPair.Cert) <= 0 {
		buf, err := ioutil.ReadFile(certsDir + "/sa.pub")
		if err != nil {
			return err
		}
		clusterConfig.SAKeyPair.Cert = buf
	}

	return nil
}

func updateCertKeyPair(keyPair *v1alpha1.KeyPair, certsDir string) error {
	if len(keyPair.Cert) <= 0 {
		buf, err := ioutil.ReadFile(certsDir + ".crt")
		if err != nil {
			return err
		}
		keyPair.Cert = buf
	}
	if len(keyPair.Key) <= 0 {
		buf, err := ioutil.ReadFile(certsDir + ".key")
		if err != nil {
			return err
		}
		keyPair.Key = buf
	}
	return nil
}

func updateClusterConfigKubeConfig(clusterConfig *v1alpha1.AzureClusterProviderSpec, tmpDirName string) error {
	kubeConfigsDir := tmpDirName + "/kubeconfigs"

	if len(clusterConfig.AdminKubeconfig) <= 0 {
		buf, err := ioutil.ReadFile(kubeConfigsDir + "/admin.conf")
		if err != nil {
			return err
		}
		clusterConfig.AdminKubeconfig = string(buf)
	}

	// // Discovery hashes typically never changes
	if len(clusterConfig.DiscoveryHashes) <= 0 {
		discoveryHashes, err := GetDiscoveryHashes(kubeConfigsDir + "/admin.conf")
		if err != nil {
			return err
		}
		clusterConfig.DiscoveryHashes = discoveryHashes
	}
	return nil
}

func CreateBastionSSHKeys(clusterConfig *v1alpha1.AzureClusterProviderSpec) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}

	if len(clusterConfig.SSHPublicKey) <= 0 {
		clusterConfig.SSHPublicKey = base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicKey))
	}
	if len(clusterConfig.SSHPrivateKey) <= 0 {
		clusterConfig.SSHPrivateKey = base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{
			Type:    "RSA PRIVATE KEY",
			Headers: nil,
			Bytes:   x509.MarshalPKCS1PrivateKey(privateKey),
		}))
	}

	return nil
}
