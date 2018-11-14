!/bin/bash
ROOT_DIR=$1
# line in the cluster config file with the resource group name
RG_LINE=16
rgName=$(cut -d ":" -f 2 <<<$(head -$RG_LINE $ROOT_DIR/generatedconfigs/cluster.yaml | tail -1) | tr -d '"')
exists=$(az group exists -n $rgName)
if [ "$exists" = true ]; then
    az group delete -n $rgName --yes
fi
