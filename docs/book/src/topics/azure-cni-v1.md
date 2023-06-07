# Using Azure CNI V1

This document aims to provide steps to configure your cluster using Azure CNI v1

- [Azure CNI v1](#azure-container-networking-interface-v1)

## Limitations

- We can only configure one subnet per control-plane node. Refer [CAPZ#3506](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3506)

## Azure Container Networking Interface v1

As of writing this document, Azure CNI needs to be installed in the following steps below.

<!-- TODO: Do we specify the number of IPs per nodes depending on the VM size? because Refer https://learn.microsoft.com/en-us/azure/aks/configure-azure-cni#maximum-pods-per-node -->

<!-- TODO: Do we specify the number of IPs per nodes depending on the VM size? As a general guideline, Microsoft recommends the following maximum number of pods per node for VM Standard_D2s_v3 and Standard_B2s using Azure CNI V1 in AKS: -->
<!-- Standard_D2s_v3: up to 30 pods per node -->
<!-- Standard_B2s: up to 10 pods per node -->
<!-- TODO: what is the diff between different Azure CNI offerings -->

## W.I.P changes

- Debug your node using

```shell
k debug node/azure-cni-v1-12484-md-0-454v6 -it --image=mcr.microsoft.com/dotnet/runtime-deps:6.0
```

- Experimenting with ip-masq-agent v2
  - `custom-config.yaml`

    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
    name: ip-masq-agent-config
    namespace: kube-system
    labels:
        component: ip-masq-agent
        kubernetes.io/cluster-service: "true"
        addonmanager.kubernetes.io/mode: EnsureExists
    data:
    ip-masq-agent: |- 
        nonMasqueradeCIDRs:
        - 10.1.0.0/24
        - 10.2.0.0/24
        masqLinkLocal: false
        masqLinkLocalIPv6: false
        
    # nonMasqueradeCIDRs: Disabling this to not override the default behavior of nomasq-all-reserved-ranges
    #   - 155.128.0.0/9
    #   - 10.240.0.0/16
    #   - 180.132.128.0/18

    # - 10.0.0.0/27 to allow max 128 (110) IPs per node to be non-Masqueraded.
    # data:
    #   ip-masq-agent: |-
    #     nonMasqueradeCIDRs:
    #       - 155.128.0.0/9
    #       - 10.240.0.0/16
    #       - 180.132.128.0/18
    #     masqLinkLocal: false
    #     masqLinkLocalIPv6: false
    ```

  - `ip-masq-agent.yaml`

    ```yaml
    # Example with two configmaps
    apiVersion: apps/v1
    kind: DaemonSet
    metadata:
    name: ip-masq-agent
    namespace: kube-system
    labels:
        component: ip-masq-agent
        kubernetes.io/cluster-service: "true"
        addonmanager.kubernetes.io/mode: Reconcile
    spec:
    selector:
        matchLabels:
        k8s-app: ip-masq-agent
    template:
        metadata:
        labels:
            k8s-app: ip-masq-agent
        spec:
        hostNetwork: true
        containers:
        - name: ip-masq-agent
            image: mcr.microsoft.com/aks/ip-masq-agent-v2:v0.1.1
            imagePullPolicy: Always
            args:
            - resync-interval=60
            - masq-chain="IP-MASQ-AGENT"
            - nomasq-all-reserved-ranges=true # All the IPs which are not marked reserved by the RFCs are masqueraded.
            - enable-ipv6=false # using the default enable-ipv6=false
            - "v=8"
            securityContext:
            privileged: false
            capabilities:
                add: ["NET_ADMIN", "NET_RAW"]
            # Uses projected volumes to merge all data in /etc/config
            volumeMounts:
            - name: ip-masq-agent-volume
                mountPath: /etc/config
                readOnly: true
        volumes:
        - name: ip-masq-agent-volume
            projected:
            sources:
                # Note these ConfigMaps must be created in the same namespace as the daemonset
                - configMap:
                    name: ip-masq-agent-config
                    optional: true
                    items:
                    - key: ip-masq-agent
                        path: ip-masq-agent
                        mode: 444
                ## since we haven't added a reconciliation process to manage the configMap
                # - configMap: 
                #     name: ip-masq-agent-config-reconciled
                #     optional: true
                #     items:
                #       # Avoiding duplicate paths
                #       - key: ip-masq-agent-reconciled
                #         path: ip-masq-agent-reconciled
                #         mode: 444
    ```

- Setting multiple secondary IPs on controlplane node is making the K8s API server take up one of the secondary IP and that is making the core-dns pods fail their readyness probe.

    ```shell
    ❯ kg endpoints kubernetes
    NAME         ENDPOINTS       AGE
    kubernetes   10.0.0.5:6443   13m
    ```

    ```shell
    Name:         azure-cni-v1-16900-control-plane-w79b9
    Namespace:    default
    .
    .
    Status:
        Addresses:
            Address:  azure-cni-v1-16900-control-plane-w79b9
            Type:     InternalDNS
            Address:  10.0.0.4
            Type:     InternalIP
            Address:  10.0.0.5
    ```
    <!-- TODO: check if ip-masq-agent can help if the user wants to set multiple IPs on the controlplane node -->
