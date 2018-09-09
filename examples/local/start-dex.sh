#!/bin/bash -e

# examplesLocalDir is required by local.properties
cwd=$(pwd)
examplesLocalDir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

usage() {
    echo
    echo "Starts an instance of dex server using a named docker container."
    echo "The name pattern is 'dex-<UUID>'"
    echo "To stop the container use:"
    echo "  docker ps -qaf name=dex- | xargs -r docker stop | xargs -r docker rm"
    echo
    echo "Usage: $0 <runtime_dir> "
    echo
    echo "  runtime_dir    parent directory to use to create the dex runtime dir."
    echo
}

if [[ $# -ne 1 ]]; then
    echo "Error: the runtime_dir parameter is required."
    usage
    exit 1
fi

# runtime is required by local.properties
runtime=$1
dexContainer=dex-$(uuidgen | tr "[:upper:]" "[:lower:]")
properties=${examplesLocalDir}/local.properties
source ${properties}

mkdir -p ${dexRuntime}

echo "== Generating DEX CA, Cert and Key"
${examplesLocalDir}/../generate-certs.sh dex ${runtime} ${properties} dex

echo "== Generating config file from template"
sed "${sedScript}" ${examplesLocalDir}/dex/config.template.yml > ${dexRuntime}/config.yml

echo "== Copying Dex templates"
#cp -r ${examplesLocalDir}/../../e2e/dextest/web ${dexRuntime}/

echo "== Starting Dex"
cmd="docker run --name ${dexContainer} -d \
    -v ${dexRuntime}:/dex \
    --net=host -p ${dexPort}:${dexPort} \
    ${dexImage} \
    serve /dex/config.yml"
echo ${cmd}
${cmd}

echo "Dex container:"
echo "${dexContainer}"
