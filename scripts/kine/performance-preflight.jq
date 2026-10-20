# Copyright 2026 The Kubernetes Authors.
# SPDX-License-Identifier: Apache-2.0

def capability($name):
  [.capabilities[]? | select(.name == $name) | .value][0];
def require($condition; $message):
  if $condition then . else error($message) end;
def sku($name):
  [$skus[0][] | select(.name == $name and .resourceType == "virtualMachines")]
  | require(length == 1; "SKU unavailable: \($name) in \($location)") | .[0]
  | require(any(.locationInfo[]?; .location == $location and any(.zones[]?; . == $zone));
            "SKU \($name) does not support zone \($zone)")
  | require(all(.restrictions[]?;
      if .type == "Location" then
        ((.values // []) | index($location)) == null
      elif .type == "Zone" then
        ((.restrictionInfo.zones // []) | index($zone)) == null
      else false end);
      "SKU \($name) is restricted for this subscription/location/zone")
  | require(capability("CpuArchitectureType") == "x64"; "SKU \($name) must be x64")
  | require(capability("AcceleratedNetworkingEnabled") == "True"; "SKU \($name) must support accelerated networking");
def node_sku($name; $role):
  sku($name)
  | require(((capability("HyperVGenerations") // "") | split(",") | index("V2")) != null;
            "\($role) SKU \($name) must support Gen2")
  # Legacy SCSI-capable SKUs omit DiskControllerTypes; reject explicitly incompatible controllers.
  | require(capability("DiskControllerTypes") == null or
      ((capability("DiskControllerTypes") | split(",") | map(gsub(" "; "")) | index("SCSI")) != null);
      "\($role) SKU \($name) is incompatible with the SCSI Azure Linux image");
def available($name; $required):
  [$quota[0][] | select((.name.value | ascii_downcase) == ($name | ascii_downcase))]
  | require(length == 1; "No quota entry for \($name)")
  | .[0]
  | ((.limit | tonumber) - (.currentValue | tonumber)) as $remaining
  | require($remaining >= $required;
            "Insufficient \($name) vCPU quota: need \($required), available \($remaining)")
  | {quota: $name, required: $required, available: $remaining};

sku($datastore) as $db
| $db
| require((capability("DiskControllerTypes") // "" | contains("NVMe")); "Datastore SKU must support NVMe")
| require(any(.locationInfo[]?; .location == $location and
    any(.zoneDetails[]?; any(.name[]?; . == $zone) and
      any(.capabilities[]?; .name == "UltraSSDAvailable" and .value == "True")));
    "Ultra Disk is not advertised for datastore SKU in zone \($zone)")
| [
    {role: "datastore", sku: $db, count: 1},
    {role: "control-plane", sku: node_sku($controlPlane; "Control-plane"), count: $controlPlanes},
    {role: "worker", sku: node_sku($worker; "Worker"), count: $workers}
  ]
| map(. + {family: .sku.family, cpus: ((.sku | capability("vCPUs") | tonumber) * .count)}) as $plan
| {
    machines: ($plan | map({role, size: .sku.name, count, family, cpus})),
    quota: ([$plan | group_by(.family)[] | available(.[0].family; (map(.cpus) | add))]
      + [available("cores"; ($plan | map(.cpus) | add))])
  }
