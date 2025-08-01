# Azure RBAC Permissions

## AKS Cluster

### Dynamically Provisioned
``` json
{{#include ./aks-dynamic-placement/rg_sub_role.json}}
```

``` json
{{#include ./aks-dynamic-placement/sub_role.json}}
```

### Statically Provisioned
``` json
{{#include ./aks-static-placement/rg_sub_role.json}}
```

``` json
{{#include ./aks-static-placement/sub_role.json}}
```

``` json
{{#include ./aks-static-placement/vnet_role.json}}
```

## IaaS Cluster

### Dynamically Provisioned
``` json
{{#include ./iaas-dynamic-placement/rg_sub_role.json}}
```

### Statically Provisioned
``` json
{{#include ./iaas-static-placement/rg_sub_role.json}}
```

``` json
{{#include ./iaas-static-placement/vnet_role.json}}
```