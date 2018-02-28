#!/bin/bash -e

# examplesLocalDir is required by local.properties
examplesLocalDir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

usage() {
    echo
    echo "Starts an instance of osprey server using a named docker container."
    echo "The name pattern is 'osprey-<UUID>'"
    echo "To stop the container use:"
    echo "  docker ps -qaf name=osprey- | xargs -r docker stop | xargs -r docker rm"
    echo
    echo "Usage: $0 <runtime_dir> "
    echo
    echo "  runtime_dir    parent directory to use to create the osprey runtime dir."
    echo
}

if [[ $# -ne 1 ]]; then
    echo "Error: the runtime_dir parameter is required."
    usage
    exit 1
fi

# runtime is required by local.properties
runtime=$1
ospreyContainer=osprey-$(uuidgen | tr "[:upper:]" "[:lower:]")
properties=${examplesLocalDir}/local.properties
source ${properties}

mkdir -p ${ospreyRuntime} ${apiserverRuntime}

echo "== Generating Osprey CA, Cert and Key"
${examplesLocalDir}/../generate-certs.sh osprey ${runtime} ${properties} osprey

echo "== Generating ApiServer CA, Cert and Key"
${examplesLocalDir}/../generate-certs.sh apiserver ${runtime}  ${properties} apiserver

echo "== Generating ospreyconfig file from template"
sed "${sedScript}" ${examplesLocalDir}/osprey/ospreyconfig.template > ${ospreyRuntime}/ospreyconfig

echo "== Generating empty kubeconfig file"
# need to create the kubeconfig file from the host, otherwise it is created by
# the container user (root) and we get `Permission denied` when trying to update it.
touch ${clusterClientKubeconfig}

echo ${ospreyRuntime}
echo "== Starting Osprey"
cmd="docker run --name ${ospreyContainer} -d \
    -v ${ospreyRuntime}:/osprey -v ${apiserverRuntime}:/apiserver -v ${dexRuntime}:/dex \
    --net=host -p ${dexPort}:${dexPort} -p ${ospreyPort}:${ospreyPort} \
    ${ospreyImage} \
    serve \
    --debug=true \
    --port=${ospreyPort} \
    --tls-cert=${ospreyTlsCert} \
    --tls-key=${ospreyTlsKey} \
    --apiServerURL=${clusterApiServerURL} \
    --apiServerCA=${clusterApiServerCA} \
    --environment=${clusterEnvironment} \
    --secret=${ospreySecret} \
    --redirectURL=${ospreyRedirectURL} \
    --issuerURL=${dexURL} \
    --issuerCA=${dexCA}"
echo ${cmd}
${cmd}

echo "Osprey container"
echo "${ospreyContainer}"
