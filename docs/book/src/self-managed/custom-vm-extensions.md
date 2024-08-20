# Custom VM Extensions

## Overview
CAPZ allows you to specify custom extensions for your Azure resources. This is useful for running custom scripts or installing custom software on your machines. You can specify custom extensions for the following resources:
 - AzureMachine
 - AzureMachinePool

## Discovering available extensions
The user is responsible for ensuring that the custom extension is compatible with the underlying image. Many VM extensions are available for use with Azure VMs. To see a complete list, use the Azure CLI command `az vm extension image list`. 

```bash
$ az vm extension image list --location westus --output table
```

## Warning
VM extensions are specific to the operating system of the VM. For example, a Linux extension will not work on a Windows VM and vice versa. See the Azure documentation for more information.
- [Virtual machine extensions and features for Linux](https://learn.microsoft.com/en-us/azure/virtual-machines/extensions/features-linux?tabs=azure-cli)
- [Virtual machine extensions and features for Windows](https://learn.microsoft.com/en-us/azure/virtual-machines/extensions/features-windows?tabs=azure-cli)

## Custom extensions for AzureMachine
To specify custom extensions for AzureMachines, you can add them to the `spec.template.spec.vmExtensions` field of your `AzureMachineTemplate`. The following fields are available:
- `name` (required): The name of the extension.
- `publisher` (required): The name of the extension publisher.
- `version` (required): The version of the extension.
- `settings` (optional): A set of key-value pairs containing settings for the extension.
- `protectedSettings` (optional): A set of key-value pairs containing protected settings for the extension. The information in this field is encrypted and decrypted only on the VM itself.

For example, the following `AzureMachineTemplate` spec specifies a custom extension that installs the `CustomScript` extension on the machine:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: test-machine-template
  namespace: default
spec:
  template:
    spec:
      vmExtensions:
      - name: CustomScript
        publisher: Microsoft.Azure.Extensions
        version: '2.1'
        settings:
          fileUris: https://raw.githubusercontent.com/me/project/hello.sh
        protectedSettings:
          commandToExecute: ./hello.sh
```

## Custom extensions for AzureMachinePool
Similarly, to specify custom extensions for AzureMachinePools, you can add them to the `spec.template.vmExtensions` field of your `AzureMachinePool`. For example, the following `AzureMachinePool` spec specifies a custom extension that installs the `CustomScript` extension on the machine:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: test-machine-pool
  namespace: default
spec:
  template:
    vmExtensions:
      - name: CustomScript
        publisher: Microsoft.Azure.Extensions
        version: '2.1'
        settings:
          fileUris: https://raw.githubusercontent.com/me/project/hello.sh
        protectedSettings:
          commandToExecute: ./hello.sh
```
