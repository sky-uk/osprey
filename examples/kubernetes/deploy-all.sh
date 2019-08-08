#!/bin/bash -e

# examplesK8sDir is required by kubernetes.properties
examplesK8sDir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

usage() {
    echo
    echo "Deploys DEX and Osprey and logs in and out of the cluster."
    echo
    echo "Usage: $0 "
    echo
    echo "  dir    directory to use to place the k8s resource files."
    echo
}

if [[ $# -ne 1 ]]; then
    echo "Error: the dir parameter is required."
    usage
    exit 1
fi

execute() {
    cmd="kubectl --context ${context} $@"
    echo ${cmd}
    ${cmd}
}

apply_global() {
    execute "apply -f $1"
}

apply_scoped() {
    execute "--namespace ${namespace} apply -f $1"
}

deployResources(){
    resources=$[@]
    for template in "${resources[@]}"; do
        sed "${sedScript}" ${examplesK8sDir}/osprey/${template} > ${ospreyRuntime}/${template}
        apply_scoped ${ospreyRuntime}/${template}
    done
}

create_secret() {
    execute "--namespace ${namespace} delete secret $1 --ignore-not-found"
    execute "--namespace ${namespace} create secret generic $1 $@"

}
check_health() {
    docker exec $1 curl -s $2 --insecure > /dev/null
    return $?
}

wait_for_app() {
    kubernetes app=$1
    kubernetes url=${app}URL
    kubernetes cacert=${app}CA
    kubernetes container=${app}Container

    if [[ -z ${!container+x} ]]; then
        echo "Error: ${app}'s container is not running:"
        docker ps
        exit 1
    fi
    docker exec ${!container} apk add --no-cache curl openssl > /dev/null
    echo "Waiting for ${app} to start:"
    while ! check_health ${!container} ${!url} ; do
        sleep 1
        echo -n "."
        if [[ $(docker inspect --format '{{.State.Status}}' ${!container}) == "exited" ]]; then
            echo
            echo "Error:failed to start ${app} container stopped unexpectedly"
            docker logs ${!container}
            exit 1
        fi
    done
}

# runtime is required by kubernetes.properties
runtime=$1
properties=${examplesK8sDir}/kubernetes.properties
source ${properties}

if [[ "x" = "${node}x" ]]; then
    echo "Error: The property 'node' is not set in ${properties}.."
    echo "Please specify the kubernetes node ip to use to contact osprey and dex."
    echo "Chose one of:"

    kubectl --context ${context} get nodes -o template --template='{{range.items}}{{range.status.addresses}}{{if eq .type "InternalIP"}}  {{.address}}:{{end}}{{end}}{{end}}' | tr ":" "\n"

    exit 1
fi

echo "== Create namespace"
mkdir -p ${runtime}
sed "${sedScript}" ${examplesK8sDir}/namespace.yml > ${runtime}/namespace.yml
apply_global ${runtime}/namespace.yml

if [[ "${ospreyAuthenticationDisabled}" = true ]]; then
    echo "== Deploy osprey (authentication disabled)"
    mkdir -p ${ospreyRuntime}
    osprey_resources=(osprey-clusterinfo.yml service.yml)
    deployResources ${osprey_resources[@]}
else
    echo "== Generate Certificates"
    mkdir -p ${sslRuntime}
    apps=(dex osprey apiserver)
    for app in "${apps[@]}"; do
        ${examplesK8sDir}/../generate-certs.sh ${app} ${runtime} ${properties} ssl
    done

    echo
    echo "== Create ssl secret"
    create_secret ${ospreySslSecret} --from-file=${sslRuntime}

    echo
    echo "== Deploy dex"
    mkdir -p ${dexRuntime}
    dex_resources=(config.yml web-templates.yml dex.yml service.yml)
    deployResources ${dex_resources[@]}

    echo
    echo "== Deploy osprey (authentication enabled)"
    mkdir -p ${ospreyRuntime}
    osprey_resources=(osprey.yml service.yml)
    deployResources ${osprey_resources[@]}
fi

echo
echo "== Generate ospreyconfig"
sed "${sedScript}" ${examplesK8sDir}/osprey/ospreyconfig.template > ${ospreyClientConfig}

echo "== Done"
echo "Osprey config in: ${runtime}/ospreyconfig"
echo "Kube config in: ${runtime}/kubeconfig"

echo "osprey --ospreyconfig ${ospreyClientConfig}" --help

