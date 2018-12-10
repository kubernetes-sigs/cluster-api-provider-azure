package e2e

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/ghodss/yaml"
	"github.com/platform9/azure-provider/pkg/cloud/azure/services"
	"golang.org/x/crypto/ssh"
	kuberand "k8s.io/apimachinery/pkg/util/rand"
	kubessh "k8s.io/kubernetes/pkg/ssh"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type Clients struct {
	kube  KubeClient
	azure services.AzureClients
}

func createTestClients() (*Clients, error) {
	kubeconfig := os.Getenv("KUBE_CONFIG")
	if kubeconfig == "" {
		return nil, fmt.Errorf("KUBE_CONFIG environment variable is not set")
	}
	kubeClient, err := NewKubeClient(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return nil, fmt.Errorf("AZURE_SUBSCRIPTION_ID environment variable is not set")
	}

	azureServicesClient, err := NewAzureServicesClient(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure services client: %v", err)
	}
	return &Clients{kube: *kubeClient, azure: *azureServicesClient}, nil
}

func machineFromConfigFile(machinesConfigPath string, configParams map[string]string) (*clusterv1.Machine, error) {
	bytes, err := ioutil.ReadFile(machinesConfigPath)
	if err != nil {
		return nil, err
	}
	content := string(bytes)
	for k, v := range configParams {
		r := regexp.MustCompile(fmt.Sprintf(`\${[%s}]*}`, k))
		content = r.ReplaceAllString(content, v)
	}

	machine := &clusterv1.Machine{}
	err = yaml.Unmarshal([]byte(content), &machine)
	if err != nil {
		return nil, err
	}

	if machine.Spec.ProviderConfig.Value == nil {
		return nil, errors.New("No ProviderConfig was found for machine")
	}

	return machine, nil
}

func genMachineParams() (params map[string]string, err error) {
	publicKey, privateKey, err := genKeyPairs()
	if err != nil {
		return nil, fmt.Errorf("error generating ssh key pairs: %v", err)
	}

	configVals := map[string]string{
		"MACHINE_NAME":    fmt.Sprintf("azure-node-%s", kuberand.String(5)),
		"SSH_PUBLIC_KEY":  base64.StdEncoding.EncodeToString(publicKey),
		"SSH_PRIVATE_KEY": base64.StdEncoding.EncodeToString(privateKey),
	}
	return configVals, nil
}

func genKeyPairs() (publicKey []byte, privateKey []byte, err error) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	public, err := ssh.NewPublicKey(&private.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	publicKeyBytes := ssh.MarshalAuthorizedKey(public)
	privateKeyBytes := kubessh.EncodePrivateKey(private)

	return publicKeyBytes, privateKeyBytes, err
}
