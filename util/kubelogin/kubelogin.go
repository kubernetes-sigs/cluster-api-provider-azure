package kubelogin

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"os"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
)

const (
	// KubeloginDefault is the default location of the kubelogin binary.
	KubeloginDefault = "kubelogin"
	// KubeloginEnvVarName is the env var name to specify a kubelogin binary location.
	KubeloginEnvVarName = "KUBELOGIN_PATH"
	// KubeConfigEnvVarName is the env var that kubelogin expects for specifying the kubeconfig file.
	KubeConfigEnvVarName = "KUBECONFIG"
)

// GetKubeloginExecutablePath returns the location of the kubelogin binary.
func GetKubeloginExecutablePath() string {
	kubeloginPath, set := os.LookupEnv(KubeloginEnvVarName)
	if set {
		return kubeloginPath
	}
	return KubeloginDefault
}

// ConvertKubeConfig converts kube-config from interactive login to non-interactive login using kubelogin.
func ConvertKubeConfig(ctx context.Context, clusterName string, configData []byte, credentialsProvider *scope.ManagedControlPlaneCredentialsProvider) (config []byte, err error) {
	if credentialsProvider == nil {
		return nil, errors.New("cannot convert kubeconfig without credential provider")
	}
	fileName := fmt.Sprintf("%s/kubeconfig-%s", os.TempDir(), clusterName)
	// kubelogin expects the "KUBECONFIG" env var to be set to the kubeconfig file path that should be converted.
	// We set it here and then restore it to its original value after the conversion.
	kubeConfigToRestore, set := os.LookupEnv(KubeConfigEnvVarName)
	defer func() {
		// Restore the kubeconfig env var if it was set before.
		if set {
			os.Setenv(KubeConfigEnvVarName, kubeConfigToRestore)
		} else {
			os.Unsetenv(KubeConfigEnvVarName)
		}
		// Delete temp file used for converting the cluster kubeconfig data.
		os.Remove(fileName)
	}()
	os.Setenv(KubeConfigEnvVarName, fileName)
	err = os.WriteFile(fileName, configData, 0644)
	if err != nil {
		return nil, err
	}
	clientSecret, err := credentialsProvider.GetClientSecret(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get client secret")
	}
	kubeLogin := GetKubeloginExecutablePath()
	err = ExecCommand(kubeLogin,
		"convert-kubeconfig", "-l", "spn", "--client-id", credentialsProvider.GetClientID(), "--client-secret",
		clientSecret, "--tenant-id", credentialsProvider.GetTenantID(), "--kubeconfig", os.Getenv(KubeConfigEnvVarName))
	if err != nil {
		return nil, errors.Wrap(err, "could not convert kubeconfig to non interactive format")
	}
	return os.ReadFile(fileName)
}
