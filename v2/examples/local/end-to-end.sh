#!/bin/bash -e

# examplesLocalDir is required by local.properties
examplesLocalDir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

usage() {
    echo
    echo "Starts DEX, Osprey and logs in and out of the cluster."
    echo
    echo "Usage: $0 "
    echo
    echo "  dir    directory to use to start the servers and place the files."
    echo
}


if [[ $# -ne 1 ]]; then
    echo "Error: the dir parameter is required."
    usage
    exit 1
fi

set +e
osprey_binary=$(which osprey 2>&1)
if [[ $? -ne 0 ]]; then
    echo ${osprey_binary}
    echo "Error: the Osprey client can't be found."
    echo "Install or download the Osprey client: https://github.com/sky-uk/osprey#installation"
    exit 1
fi
set -e

stop_app() {
    local app=$1
    local container=$2
    if [[ $# -ne 2 ]] || [[ -z ${container+x} ]]; then
        return 0
    fi
    echo
    echo "== Removing ${app} container ${container}"
    echo "Server logs:" >> $(log_file ${app})
    docker logs ${container} >> $(log_file ${app}) 2>&1
    docker stop ${container} | xargs docker rm >> $(log_file ${app})
}

cleanup() {
    stop_app osprey ${ospreyContainer}
    stop_app dex ${dexContainer}
}
trap "{ CODE=$?; cleanup ; exit ${CODE}; }" EXIT


osprey() {
    set +e
    local cmd="${osprey_binary} --ospreyconfig ${ospreyClientConfig} $@"
    echo ${cmd}
    echo
    ${cmd}
    echo
    sleep 2
    set -e
}

log_file() {
    echo ${runtime}/$1.log
}

check_health() {
    docker exec $1 curl -s $2 --insecure > /dev/null
    return $?
}

wait_for_app() {
    local app=$1
    local url=${app}URL
    local cacert=${app}CA
    local container=${app}Container

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

containerId() {
    local containerName=$1
    if [[ -z ${containerName+x} ]]; then
    echo "empty container name"
        return -1
    fi
    docker ps -qaf name=${containerName}
}

# runtime is required by local.properties
runtime=$1
properties=${examplesLocalDir}/local.properties
source ${properties}

mkdir -p ${runtime}

echo
echo "== Starting Dex"
${examplesLocalDir}/start-dex.sh ${runtime} > $(log_file dex)
dexContainer=$(containerId $(tail -1 $(log_file dex)))
echo "container: ${dexContainer}"
wait_for_app dex

echo
echo "== Starting Osprey"
${examplesLocalDir}/start-osprey.sh ${runtime} > $(log_file osprey)
ospreyContainer=$(containerId $(tail -1 $(log_file osprey)))
echo "container: ${ospreyContainer}"
wait_for_app osprey

echo
echo "== Log in"
osprey user login

echo "== User details: logged in"
osprey user

echo "== Log out"
osprey user logout

echo "== User details: logged out"
osprey user

echo "== Done"
cp ${clusterClientKubeconfig} ${runtime}/kubeconfig
echo "Osprey config in: ${runtime}/ospreyconfig"
echo "Kube config in: ${runtime}/kubeconfig"
echo "DEX logs in: $(log_file dex)"
echo "Osprey logs in: $(log_file osprey)"
