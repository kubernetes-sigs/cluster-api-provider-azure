---
title: Bootstrap failure detection
authors:
  - "@CecileRobertMichon"
  - "@jackfrancis"
reviewers:
  - @devigned
  - @nader-ziada
creation-date: 2020-07-28
last-updated: 2020-12-14
status: implementable
see-also:
- https://github.com/kubernetes-sigs/cluster-api/issues/2554
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/603
---


# Bootstrap failure detection


## Summary

The status of VM bootstrap operations (cloud-init, kubeadm) is opaque from the perspective of cluster-api resources that represent those VMs.

## Motivation

### Goals
- Assist programmatic consumers and end users in determining when and why bootstrapping failed
- Enable management clusters to determine bootstrapping status
- Solve the Azure provider following generic bootstrap status interfaces defined by cluster-api
- Be compatible with all bootstrap providers
- Work for both Linux and Windows

### Non-Goals / Future Work
- Implement full cloud-init/kubeadm log stream data as a part of the cluster-api/capz resource.

## Available options for Cluster API Provider Azure

### Option 1: Enable VM boot diagnostics
Azure VM and VMSS support a boot diagnostics feature which streams cloud init logs and boot time output into a storage account. This would allow log collection for some aspects of bootstrapping (at least cloud init logs).
See https://learn.microsoft.com/azure/virtual-machines/boot-diagnostics and https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/606

#### Enable VM Boot Diagnostics Pros:
- Low effort (this VM feature gets us basic logs for free once we enable it)
- All details of VM and OS bootstrapping are persisted
- Available for Linux and Windows
- No control over the way the logs output is rendered

#### Enable VM Boot Diagnostics Cons:
- Requires a storage account
- Add’l IaaS cost
- Would not expose diagnostics to cluster components, have to consume directly from the VM
- Can’t really use this option exclusively to solve the “determine bootstrap status” goal, would have to be used in concert with additional controller implementation that synthesizes the boot diagnostics output

### Option 2: Pub/Sub model using Azure Service Bus
Similar to the "Simple Notification Service & Simple Queue Service" solution for AWS above.
For more info see https://learn.microsoft.com/azure/service-bus-messaging/ and https://github.com/Azure/azure-service-bus-go

#### Pub Sub Pros:
- Extensible, can support lots of client patterns, e.g.:
- Bootstrap process publishes logs and mgmt cluster consumes them
- AzureMachine reconciliation can detect and possibly deal with certain failure conditions

#### Pub Sub Cons:
- Complicated, lots of additional moving pieces:
- Might have to write our own OS-specific tools to consume bootstrap logs and publish them, and those tools would be installed by additional VM Extensions, or add'l cloud-init configuration
- Add’l IaaS cost

### Option 3: Azure Custom Script Extensions
The Custom Script Extension downloads and executes scripts on Azure virtual machines (and VMSS instances). We could leverage extensions to either 1) run kubeadm init/join commands (ie. move the "runcmd" content from cloud init to   a custom script extension). This is useful because you can control the exit code for VM Extensions which allows for better error reporting than cloud init. The max script size is also 256 KB (vs 64 KB for user data). This does not collect logs, but part of the extension could be to export the logs externally (to a storage account for example). The extension could also be used purely for checking bootstrapping status (ie. cloud init runcmd still runs the init/join) and exporting logs.
https://learn.microsoft.com/azure/virtual-machines/extensions/custom-script-linux
https://learn.microsoft.com/azure/virtual-machines/extensions/custom-script-windows

#### Custom Script Extension Pros:
- Generic and flexible for both Linux and Windows, can basically execute any arbitrary code (so long as it’s under the size limits defined above)

#### Custom Script Extension Cons:
- You may only have one Custom Script Extension per VM, so using this interface to implement bootstrap failure detection means we are not able to expose the Custom Script Extension VM feature to users as a “general purpose” script interface

### Option 4: VM run command
https://learn.microsoft.com/azure/virtual-machines/linux/run-command
This is similar to the above idea of using a custom script extension but instead of deploying an additional VM extension resource, the Run Command feature uses the virtual machine (VM) agent to run shell scripts within an Azure VM. This works without requiring RDP/SSH access to the VM.

#### VM runcmd Pros:
- Relatively simple; we already have mature Azure SDK patterns in the AzureMachine controller implementation, would not be that much additional work to incorporate a runcmd operation against node vms

#### VM runcmd Cons:
- Limited stdout from runcmd request output
- As with the VM Boot Diagnostics solution, this requires the AzureMachine controller to actually synthesize the runcmd (or multiple runcmd) result(s) into a terminal state outcome

### Option 5: Custom capz-specific Azure VM Extension (recommended)
A custom Azure VM Extension is basically a unit of foo that does a very finite set of things on a VM as it bootstraps itself. We could use this to implement a set of capz-focused bootstrap failure reporting, to support both investigation and remediation.

#### Custom capz VM Extension Pros:
- The same generic flexibility (although rendered as a concrete solution, not at runtime) as the Custom Script Extension option above, but maintains availability for future, user-configurable Custom Script Extensions solutions
- Exposes a convenient binary success/failure as part of the Azure VM resource itself
- Easy to query from the AzureMachine controller
- Easy for users to introspect via Azure APIs, CLI, portal
- Named property allows for convenient disambiguation from generic Azure (or other non-capz-related) errors

#### Custom capz VM Extension Cons:
- Non-trivial (though one-time) administrative overhead to create a named Azure VM Extension
- Ongoing maintenance requires a separate release workflow compared to capz (in other words, we don’t simply ship changes to this *with* capz releases)

### Option 6: postKubeadmCommand
This is an option that could be used for kubeadm bootstrap-provided solutions only, which exposes an array interface to execute a set of arbitrary, serialized shell statements (essentially a thin wrapper around cloud-init’s runcmd interface) after kubeadm finishes.

#### postKubeadmCommand Pros:
- The interface is already present, assuming we only want to solve this for the kubeadm bootstrap provider

#### postKubeadmCommand Cons:
- *only* works for kubeadm bootstrap provider
- Sort of breaks the UX contract for postKubeadmCommand, which is intended to be a user-configurable interface and not reserved for use by cluster-api controllers

## Conclusions
A few conclusions surfaced when exploring these options:

1. Evaluating simple success/failure of VM bootstrapping is most easily done on the VM itself, because under no scenarios is there an option *not* to source some of the relevant input data from the VM. And because we can’t avoid establishing a connection to the VM’s filesystem, it simplifies things greatly to do that locally via a process/daemon running on the VM.
2. The actual implementation that determines “did I bootstrap successfully?” should be defined by each bootstrap provider, as each provider has its own files/operational conditions to validate. The validation on the Azure side should be as minimal as possible and delegate all responsibility of running checks to the bootstrap provider.
3. We need to support Linux and Windows, and though there is one convenience (VM Boot Diagnostics) that may allow us to get a common result across both OSes “for free”, in practice there is enough heterogeneity at all layers (VM, OS, potentially even capi) that we should expect to have to maintain a discrete set of implementations for each platform. So we want to choose a solution that makes supporting both Linux and Windows distinctly natural.

The most sensible solution would be to reuse the existing CustomScriptExtension interface that can be attached to both Windows and Linux VMs. But the fact that VMs may only support a single CustomScriptExtension is a non-trivial problem, as it removes that configuration vector for users. That vector can be a powerful configuration option — paired with custom OS images — to deliver regular runtime functionality to the underlying Azure VM running as a Kubernetes node. In particular during emergency scenarios being able to “patch” your node’s Azure VM implementation quickly using this interface can save a user many hours if he/she had to otherwise wait for a new OS image, or worse, a new VHD publication.

So, given that we don’t want to “reserve” the CustomScriptExtension VM interface for capz, thus preventing users from using it more generically and flexibly (as it’s intended to be used), we want to propose curating a capz-specific Azure VM Extension dedicated to running on the VM during provisioning and evaluating the success/fail state of its bootstrap operation(s) towards joining a capz-enabled Kubernetes cluster.

At a very high level, this is what we want our capz-named Azure VM Extension to do:

- Wait for a configurable time duration to validate the minimum necessary to determine bootstrap success/fail
  - This would require updating the CAPI bootstrap provider contract to include a signal (such a sentinel file) on the VM to indicate that all bootstrap operations have finished successfully
- When terminal state has been reached, return an appropriate exit code to the Azure VM Extension itself
  - At a minimum we will return a binary (e.g., 0 for success, 1 for failure) exit state
- If a terminal state has not been reached before the configurable timeout has been reached, return an appropriate failure exit code
  - Again, we assume using a common exit code for all failure states is acceptable for the initial scope of this work
- Set appropriate AzureMachine (and possibly Machine?) conditions

VM Boot Diagnostics should be used in conjunction with the extension. The VM extension provides a simple pass/fail signal that can be used by CAPZ to set conditions and indicate bootstrap status. Boot Diagnostics can provide a quick look at what went wrong to the user by displaying cloud-init logs without needing to SSH into the VM. In the future, boot diagnostics might even used to stream logs programmatically at the AzureMachine level.


## Questions
- Can the custom Azure VM Extension be overloaded to solve for both the Windows and Linux case at runtime? In other words, can we publish a single Extension that will be able to easily choose the Windows or Linux path depending upon the OS type of the VM it’s attached to?
    - No, a separate extension has to be published for Linux and Windows.
- Will the extension code be open-source?
    - Yes, the CAPZ extension will be a clone of the [custom script extension](https://github.com/Azure/custom-script-extension-linux).
- Will the extension need to be republished often?
    - No. Once the extension is published once, we don't expect to have to republish it unless code defects are found in the extension itself. The script run by the extension will live in the cluster-api-provider-azure repository and can be updated without changing the extension itself.
- Will the extension be available in all Azure regions and clouds?
    - Yes. At first, the extension will be available in all Azure Public Cloud regions. Shortly after, it will be published in other clouds.
- Does this proposed solution work for both VMs and VMSS?
    - Yes. Scale sets have can have a common extension that runs on all instances.
