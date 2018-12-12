#!/bin/bash
ROOT_DIR=$1
SSH_USER="ClusterAPI"
# line in the cluster config file with the resource group name
RG_LINE=16

echo "Adding keys to agent"
eval `ssh-agent -s`
ssh-add $ROOT_DIR/cmd/clusterctl/examples/azure/out/sshkey

rgName=$(cut -d ":" -f 2 <<<$(head -$RG_LINE $ROOT_DIR/cmd/clusterctl/examples/azure/out/cluster.yaml | tail -1) | tr -d '"')

# get the master node ip address
echo "Looking for master VM in resource group: $rgName"
ip=$(az vm list-ip-addresses -g $rgName --query '[*].virtualMachine.network.publicIpAddresses[?starts_with(name, `ClusterAPIIP-azure-master`) == `true`].ipAddress | [0][0]' | tr -d '"')
echo "Getting kube config from vm with ip: $ip"
scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null ClusterAPI@$ip:/home/ClusterAPI/.kube/config $ROOT_DIR/kubeconfig
