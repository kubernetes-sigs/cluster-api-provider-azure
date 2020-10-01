# -*- mode: Python -*-

envsubst_cmd = "./hack/tools/bin/envsubst"

update_settings(k8s_upsert_timeout_secs=60)  # on first tilt up, often can take longer than 30 seconds

# set defaults
settings = {
    "allowed_contexts": [
        "kind-capz"
    ],
    "deploy_cert_manager": True,
    "preload_images_for_kind": True,
    "kind_cluster_name": "capz",
    "capi_version": "v0.3.10",
    "cert_manager_version": "v0.16.1",
    "kubernetes_version": "v1.18.8",
    "aks_kubernetes_version": "v1.17.7"
}

keys = ["AZURE_SUBSCRIPTION_ID_B64", "AZURE_TENANT_ID_B64", "AZURE_CLIENT_SECRET_B64", "AZURE_CLIENT_ID_B64"]

# global settings
settings.update(read_json(
    "tilt-settings.json",
    default = {},
))

if settings.get("trigger_mode") == "manual":
    trigger_mode(TRIGGER_MODE_MANUAL)

if "allowed_contexts" in settings:
    allow_k8s_contexts(settings.get("allowed_contexts"))

if "default_registry" in settings:
    default_registry(settings.get("default_registry"))


# Prepull all the cert-manager images to your local environment and then load them directly into kind. This speeds up
# setup if you're repeatedly destroying and recreating your kind cluster, as it doesn't have to pull the images over
# the network each time.
def deploy_cert_manager():
    registry = settings.get("cert_manager_registry", "quay.io/jetstack")
    version = settings.get("cert_manager_version")

    # check if cert-mamager is already installed, otherwise pre-load images & apply the manifest
    # NB. this is required until https://github.com/jetstack/cert-manager/issues/3121 is addressed otherwise
    # when applying the manifest twice to same cluster kubectl get stuck
    existsCheck = str(local("kubectl get namespaces"))
    if existsCheck.find("cert-manager") == -1:
        # pre-load cert-manager images in kind
        images = ["cert-manager-controller", "cert-manager-cainjector", "cert-manager-webhook"]
        if settings.get("preload_images_for_kind"):
            for image in images:
                local("docker pull {}/{}:{}".format(registry, image, version))
                local("kind load docker-image --name {} {}/{}:{}".format(settings.get("kind_cluster_name"), registry, image, version))

        # apply the cert-manager manifest
        local("kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/{}/cert-manager.yaml".format(version))

    # verifies cert-manager is properly working (https://cert-manager.io/docs/installation/kubernetes/#verifying-the-installation)
    # 1. wait for the cert-manager to be running
    local("kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment/cert-manager")
    local("kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment/cert-manager-cainjector")
    local("kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment/cert-manager-webhook")

    # 2. create a test certificate
    local("cat << EOF | kubectl apply -f - " + cert_manager_test_resources + "EOF")
    local("kubectl wait --for=condition=Ready --timeout=300s -n cert-manager-test certificate/selfsigned-cert ")
    local("cat << EOF | kubectl delete -f - " + cert_manager_test_resources + "EOF")


# deploy CAPI
def deploy_capi():
    version = settings.get("capi_version")
    capi_uri = "https://github.com/kubernetes-sigs/cluster-api/releases/download/{}/cluster-api-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | kubectl apply -f -".format(capi_uri, envsubst_cmd)
    local(cmd, quiet=True)
    if settings.get("extra_args"):
        extra_args = settings.get("extra_args")
        if extra_args.get("core"):
            core_extra_args = extra_args.get("core")
            if core_extra_args:
                for namespace in ["capi-system", "capi-webhook-system"]:
                    patch_args_with_extra_args(namespace, "capi-controller-manager", core_extra_args)
                patch_capi_manager_role_with_exp_infra_rbac()
        if extra_args.get("kubeadm-bootstrap"):
            kb_extra_args = extra_args.get("kubeadm-bootstrap")
            if kb_extra_args:
                patch_args_with_extra_args("capi-kubeadm-bootstrap-system", "capi-kubeadm-bootstrap-controller-manager", kb_extra_args)


def patch_args_with_extra_args(namespace, name, extra_args):
    args_str = str(local('kubectl get deployments {} -n {} -o jsonpath={{.spec.template.spec.containers[1].args}}'.format(name, namespace)))
    args_to_add = [arg for arg in extra_args if arg not in args_str]
    if args_to_add:
        args = args_str[1:-1].split()
        args.extend(args_to_add)
        patch = [{
            "op": "replace",
            "path": "/spec/template/spec/containers/1/args",
            "value": args,
        }]
        local("kubectl patch deployment {} -n {} --type json -p='{}'".format(name, namespace, str(encode_json(patch)).replace("\n", "")))


# patch the CAPI manager role to also provide access to experimental infrastructure
def patch_capi_manager_role_with_exp_infra_rbac():
    api_groups_str = str(local('kubectl get clusterrole capi-manager-role -o jsonpath={.rules[1].apiGroups}'))
    exp_infra_group = "exp.infrastructure.cluster.x-k8s.io"
    if exp_infra_group not in api_groups_str:
        groups = api_groups_str[1:-1].split() # "[arg1 arg2 ...]" trim off the first and last, then split
        groups.append(exp_infra_group)
        patch = [{
            "op": "replace",
            "path": "/rules/1/apiGroups",
            "value": groups,
        }]
        local("kubectl patch clusterrole capi-manager-role --type json -p='{}'".format(str(encode_json(patch)).replace("\n", "")))


# Users may define their own Tilt customizations in tilt.d. This directory is excluded from git and these files will
# not be checked in to version control.
def include_user_tilt_files():
    user_tiltfiles = listdir("tilt.d")
    for f in user_tiltfiles:
        include(f)


def append_arg_for_container_in_deployment(yaml_stream, name, namespace, contains_image_name, args):
    for item in yaml_stream:
        if item["kind"] == "Deployment" and item.get("metadata").get("name") == name and item.get("metadata").get("namespace") == namespace:
            containers = item.get("spec").get("template").get("spec").get("containers")
            for container in containers:
                if contains_image_name in container.get("image"):
                    container.get("args").extend(args)


def fixup_yaml_empty_arrays(yaml_str):
    yaml_str = yaml_str.replace("conditions: null", "conditions: []")
    return yaml_str.replace("storedVersions: null", "storedVersions: []")


def validate_auth():
    substitutions = settings.get("kustomize_substitutions", {})
    missing = [k for k in keys if k not in substitutions]
    if missing:
        fail("missing kustomize_substitutions keys {} in tilt-setting.json".format(missing))

tilt_helper_dockerfile_header = """
# Tilt image
FROM golang:1.13.15 as tilt-helper
# Support live reloading with Tilt
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh
"""

tilt_dockerfile_header = """
FROM gcr.io/distroless/base:debug as tilt
WORKDIR /
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY manager .
"""

cert_manager_test_resources = """
apiVersion: v1
kind: Namespace
metadata:
  name: cert-manager-test
---
apiVersion: cert-manager.io/v1alpha2
kind: Issuer
metadata:
  name: test-selfsigned
  namespace: cert-manager-test
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1alpha2
kind: Certificate
metadata:
  name: selfsigned-cert
  namespace: cert-manager-test
spec:
  dnsNames:
    - example.com
  secretName: selfsigned-cert-tls
  issuerRef:
    name: test-selfsigned
"""

# Build CAPZ and add feature gates
def capz():
    # Apply the kustomized yaml for this provider
    substitutions = settings.get("kustomize_substitutions", {})
    os.environ.update(substitutions)
    yaml = str(kustomizesub("./config"))


    # add extra_args if they are defined
    if settings.get("extra_args"):
        azure_extra_args = settings.get("extra_args").get("azure")
        if azure_extra_args:
            yaml_dict = decode_yaml_stream(yaml)
            append_arg_for_container_in_deployment(yaml_dict, "capz-controller-manager", "capz-system", "cluster-api-azure-controller", azure_extra_args)
            yaml = str(encode_yaml_stream(yaml_dict))
            yaml = fixup_yaml_empty_arrays(yaml)

    # Set up a local_resource build of the provider's manager binary.
    local_resource(
        "manager",
        cmd = 'mkdir -p .tiltbuild;CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags \'-extldflags "-static"\' -o .tiltbuild/manager',
        deps = ["api", "cloud", "config", "controllers", "exp", "feature", "pkg", "go.mod", "go.sum", "main.go"]
    )

    k8s_resource('capz-controller-manager:deployment:capz-system', objects=[
        'azureclusters.infrastructure.cluster.x-k8s.io:customresourcedefinition',
        'azuremachinepools.exp.infrastructure.cluster.x-k8s.io:customresourcedefinition',
        'azuremachines.infrastructure.cluster.x-k8s.io:customresourcedefinition',
        'azuremachinetemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition',
        'azuremanagedclusters.exp.infrastructure.cluster.x-k8s.io:customresourcedefinition',
        'azuremanagedcontrolplanes.exp.infrastructure.cluster.x-k8s.io:customresourcedefinition',
        'azuremanagedmachinepools.exp.infrastructure.cluster.x-k8s.io:customresourcedefinition',
    ])

    k8s_resource('capz-controller-manager:deployment:capi-webhook-system', objects=[
        'capz-validating-webhook-configuration:validatingwebhookconfiguration',
        'capz-mutating-webhook-configuration:mutatingwebhookconfiguration',
    ])

    dockerfile_contents = "\n".join([
        tilt_helper_dockerfile_header,
        tilt_dockerfile_header,
    ])

    entrypoint = ["sh", "/start.sh", "/manager"]
    extra_args = settings.get("extra_args")
    if extra_args:
        entrypoint.extend(extra_args)

    # Set up an image build for the provider. The live update configuration syncs the output from the local_resource
    # build into the container.
    docker_build(
        ref = "gcr.io/k8s-staging-cluster-api-azure/cluster-api-azure-controller",
        context = "./.tiltbuild/",
        dockerfile_contents = dockerfile_contents,
        target = "tilt",
        entrypoint = entrypoint,
        only = "manager",
        live_update = [
            sync(".tiltbuild/manager", "/manager"),
            run("sh /restart.sh"),
        ],
        ignore = ["templates"]
    )

    k8s_yaml(blob(yaml))

def calico_crs():
    local("kubectl delete configmaps calico-addon --ignore-not-found=true")
    local("kubectl create configmap calico-addon --from-file=templates/addons/calico.yaml")
    local("kubectl delete configmaps calico-ipv6-addon --ignore-not-found=true")
    local("kubectl create configmap calico-ipv6-addon --from-file=templates/addons/calico-ipv6.yaml")
    local("kubectl apply -f templates/addons/calico-resource-set.yaml")

# run worker clusters specified from 'tilt up' or in 'tilt_config.json'
def flavors():
    config.define_string_list("templates-to-run", args=True)
    config.define_string_list("worker-flavors")
    cfg = config.parse()
    worker_templates = cfg.get('templates-to-run', [])

    substitutions = settings.get("kustomize_substitutions", {})
    for key in keys:
        if key[-4:] == "_B64":
            substitutions[key[:-4]] = base64_decode(substitutions[key])

    ssh_pub_key = "AZURE_SSH_PUBLIC_KEY_B64"
    ssh_pub_key_path = "~/.ssh/id_rsa.pub"
    if not substitutions.get(ssh_pub_key):
        print("{} was not specified in tilt_config.json, attempting to load {}".format(ssh_pub_key, ssh_pub_key_path))
        substitutions[ssh_pub_key] = base64_encode_file(ssh_pub_key_path)

    for flavor in cfg.get("worker-flavors", []):
        if flavor not in worker_templates:
            worker_templates.append(flavor)
    for flavor in worker_templates:
        deploy_worker_templates(flavor, substitutions)


def deploy_worker_templates(flavor, substitutions):
    # validate flavor exists
    if flavor == "default":
        yaml_file = "./templates/cluster-template.yaml"
    else:
        yaml_file = "./templates/cluster-template-" + flavor + ".yaml"
        if not os.path.exists(yaml_file):
            fail(yaml_file + " not found")

    yaml = str(read_file(yaml_file))

    # azure account and ssh replacements
    for substitution in substitutions:
        value = substitutions[substitution]
        yaml = yaml.replace("${" + substitution + "}", value)

    # if metadata defined for worker-templates in tilt_settings
    if "worker-templates" in settings:
        # first priority replacements defined per template
        if "flavors" in settings.get("worker-templates", {}):
            substitutions = settings.get("worker-templates").get("flavors").get(flavor, {})
            for substitution in substitutions:
                value = substitutions[substitution]
                yaml = yaml.replace("${" + substitution + "}", value)

        # second priority replacements defined common to templates
        if "metadata" in settings.get("worker-templates", {}):
            substitutions = settings.get("worker-templates").get("metadata", {})
            for substitution in substitutions:
                value = substitutions[substitution]
                yaml = yaml.replace("${" + substitution + "}", value)

    # programmatically define any remaining vars
    substitutions = {
        "CLUSTER_NAME": flavor + "-template",
        "AZURE_LOCATION": "eastus",
        "AZURE_VNET_NAME": flavor + "-template-vnet",
        "AZURE_RESOURCE_GROUP": flavor + "-template-rg",
        "CONTROL_PLANE_MACHINE_COUNT": "1",
        "KUBERNETES_VERSION": settings.get("kubernetes_version"),
        "AZURE_CONTROL_PLANE_MACHINE_TYPE": "Standard_D2s_v3",
        "WORKER_MACHINE_COUNT": "2",
        "AZURE_NODE_MACHINE_TYPE": "Standard_D2s_v3",
    }

    if flavor == "aks":
        # AKS version support is usually a bit behind CAPI version, so use an older version
        substitutions["KUBERNETES_VERSION"] = settings.get("aks_kubernetes_version")

    for substitution in substitutions:
        value = substitutions[substitution]
        yaml = yaml.replace("${" + substitution + "}", value)

    yaml = envsubst(yaml)
    yaml = yaml.replace('"', '\\"')     # add escape character to double quotes in yaml

    local_resource(
        "worker-" + flavor,
        cmd = "make generate-flavors; echo \"" + yaml + "\" > ./.tiltbuild/worker-" + flavor + ".yaml; cat ./.tiltbuild/worker-" + flavor + ".yaml | " + envsubst_cmd + " | kubectl apply -f -",
        auto_init = False,
        trigger_mode = TRIGGER_MODE_MANUAL
    )


def base64_encode(to_encode):
    encode_blob = local("echo '{}' | tr -d '\n' | base64 - | tr -d '\n'".format(to_encode), quiet=True)
    return str(encode_blob)


def base64_encode_file(path_to_encode):
    encode_blob = local("cat {} | tr -d '\n' | base64 - | tr -d '\n'".format(path_to_encode), quiet=True)
    return str(encode_blob)


def base64_decode(to_decode):
    decode_blob = local("echo '{}' | base64 --decode -".format(to_decode), quiet=True)
    return str(decode_blob)

def envsubst(yaml):
    yaml = yaml.replace('"', '\\"')
    return str(local("echo \"{}\" | {}".format(yaml, envsubst_cmd), quiet=True))

def kustomizesub(folder):
    yaml = local('hack/kustomize-sub.sh {}'.format(folder), quiet=True)
    return yaml

##############################
# Actual work happens here
##############################

validate_auth()

include_user_tilt_files()

if settings.get("deploy_cert_manager"):
    deploy_cert_manager()

deploy_capi()

capz()

calico_crs()

flavors()
