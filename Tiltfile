# -*- mode: Python -*-

# set defaults

settings = {
    "allowed_contexts": [
        "kind-capz"
    ],
    "deploy_cert_manager": True,
    "preload_images_for_kind": True,
    "kind_cluster_name": "capz",
    "capi_version": "v0.3.5",
    "cert_manager_version": "v0.11.0",
    "feature_gates": '--feature-gates MachinePool=true'
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
    registry = "quay.io/jetstack"
    version = settings.get("cert_manager_version")
    images = ["cert-manager-controller", "cert-manager-cainjector", "cert-manager-webhook"]

    if settings.get("preload_images_for_kind"):
        for image in images:
            local("docker pull {}/{}:{}".format(registry, image, version))
            local("kind load docker-image --name {} {}/{}:{}".format(settings.get("kind_cluster_name"), registry, image, version))

    local("kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/{}/cert-manager.yaml".format(version))

    # wait for the service to become available
    local("kubectl wait --for=condition=Available --timeout=300s apiservice v1beta1.webhook.cert-manager.io")


# deploy CAPI
def deploy_capi():
    version = settings.get("capi_version")
    local("kubectl apply -f https://github.com/kubernetes-sigs/cluster-api/releases/download/{}/cluster-api-components.yaml".format(version))
    if settings.get("feature_gates"):
        args_str = str(local('kubectl get deployments capi-controller-manager -n capi-system -o jsonpath={.spec.template.spec.containers[1].args}'))
        if settings.get("feature_gates") not in args_str:
            args = args_str[1:-1].split() # "[arg1 arg2 ...]" trim off the first and last, then split
            args.append("\"{}\"".format(settings.get("feature_gates")))
            patch = [{
                "op": "replace",
                "path": "/spec/template/spec/containers/1/args",
                "value": args,
            }]
            local("kubectl patch deployment capi-controller-manager -n capi-system  --type json -p='{}'".format(str(encode_json(patch)).replace("\n", "")))


# Users may define their own Tilt customizations in tilt.d. This directory is excluded from git and these files will
# not be checked in to version control.
def include_user_tilt_files():
    user_tiltfiles = listdir("tilt.d")
    for f in user_tiltfiles:
        include(f)


def append_arg_for_container_in_deployment(yaml_stream, name, namespace, contains_image_name, arg):
    for item in yaml_stream:
        if item["kind"] == "Deployment" and item.get("metadata").get("name") == name and item.get("metadata").get("namespace") == namespace:
            containers = item.get("spec").get("template").get("spec").get("containers")
            for container in containers:
                if contains_image_name in container.get("image"):
                    container.get("args").append("\"{}\"".format(arg))


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
FROM golang:1.13.8 as tilt-helper
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

# Build CAPZ and add feature gates
def capz():
    # Apply the kustomized yaml for this provider
    yaml = str(kustomize("./config"))
    substitutions = settings.get("kustomize_substitutions", {})
    for substitution in substitutions:
        value = substitutions[substitution]
        yaml = yaml.replace("${" + substitution + "}", value)

    # add feature gates if they are defined
    if settings.get("feature_gates"):
        yaml_dict = decode_yaml_stream(yaml)
        append_arg_for_container_in_deployment(yaml_dict, "capz-controller-manager", "capz-system", "cluster-api-azure-controller", settings.get("feature_gates"))
        yaml = str(encode_yaml_stream(yaml_dict))
        yaml = fixup_yaml_empty_arrays(yaml)

    # Set up a local_resource build of the provider's manager binary.
    local_resource(
        "manager",
        cmd = 'mkdir -p .tiltbuild;CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags \'-extldflags "-static"\' -o .tiltbuild/manager',
        deps = ["./api", "./main.go", "./pkg", "./controllers", "./cloud", "./exp"]
    )

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
            sync("./.tiltbuild/manager", "/manager"),
            run("sh /restart.sh"),
        ],
    )

    k8s_yaml(blob(yaml))

##############################
# Actual work happens here
##############################

validate_auth()

include_user_tilt_files()

if settings.get("deploy_cert_manager"):
    deploy_cert_manager()

deploy_capi()

capz()