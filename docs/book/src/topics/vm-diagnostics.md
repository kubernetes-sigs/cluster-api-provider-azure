# VM Diagnostics

This document describes how configure the VM Diagnostics options in Azure.

The VM Diagnostics allow you to configure options for troubleshooting the startup of an Azure Virtual Machine.
You can use this feature to investigate boot failures for custom or platform images.

For more information on this feature, see [here](https://learn.microsoft.com/azure/virtual-machines/boot-diagnostics#boot-diagnostics-view).

## Configuring Diagnostics

Boot Diagnostics for the VM can enabled, disabled, and configured based on user preference to be either managed by Azure or managed independently by the user.
Here is a description of the feature's fields and the available configuration options for them:
```yaml
kind: AzureMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
  template:
    spec:
      [...]
      diagnostics: # defines a diagnostics configuration block
        boot: # defines a boot diagnostics configuration block
          storageAccountType: Managed | UserManaged | Disabled # defaults to Managed for backwards compatibility
          userManaged: # This is only valid to be set when the account type is UserManaged.
            storageAccountURI: "<your-storage-URI>"
```

## Example

The below example shows how to enable boot diagnostics and configure a Managed (by Azure) storage for them.

```yaml
kind: AzureMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
  template:
    spec:
      [...]
      diagnostics:
        boot:
           storageAccountType: Managed # defaults to Managed for backwards compatibility
```

The below example shows how to enable boot diagnostics and configure user-managed storage (with a custom Storage URI) for them.
```yaml
kind: AzureMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
  template:
    spec:
      [...]
      diagnostics:
        boot:
           storageAccountType: UserManaged
           userManaged:
             storageAccountURI: "<your-storage-URI>"
```

The below example shows how to disable boot diagnostics.
```yaml
kind: AzureMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
  template:
    spec:
      [...]
      diagnostics:
        boot:
           storageAccountType: Disabled
```
