#!/bin/bash -e

scriptDir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

usage() {
    echo
    echo "Creates a self signed certificate for TLS."
    echo
    echo "Usage: $0 <app> <example_dir> <properties> <cert_dir>"
    echo
    echo "  app           name of the app to generate the certificate for: dex | osprey | apiserver"
    echo "  example_dir   path to the root of the example dir"
    echo "  properties    path to the properties file to load"
    echo "  cert_dir      path to the subdir in example_Dir to generate the files in"
    echo
}

if [[ $# -ne 4 ]]; then
    echo "Error: the app and target_dir are required."
    usage
    exit 1
fi

app=$1
if [[ "${app}" != "dex" ]] && [[ "${app}" != "osprey" ]] && [[ "${app}" != "apiserver" ]] ; then
    echo "Error: invalid app name."
    usage
    exit 1
fi

# required by properties
runtime=$2
properties=$3
source ${properties}

certDir=${runtime}/$4
mkdir -p ${certDir}

# cluster.environment comes from local.properties
appCN=${app}.${clusterEnvironment:-foo.cluster}
cat << EOF > ${certDir}/${app}-req.cnf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${appCN}
DNS.2 = ${app-example}
DNS.3 = localhost
IP.1 = 127.0.0.1
IP.2 = ${node}
EOF

echo "Generating ${app}'s CA Key & Certificate"
openssl genrsa -out ${certDir}/${app}-ca-key.pem 2048 > /dev/null 2>&1
openssl req -x509 -new -nodes -key ${certDir}/${app}-ca-key.pem -days 10 -out ${certDir}/${app}-ca.pem -subj "/CN=${app}-ca"  > /dev/null 2>&1

echo "Generating ${app}'s Client Key"
openssl genrsa -out ${certDir}/${app}-key.pem 2048  > /dev/null 2>&1
openssl req -new -key ${certDir}/${app}-key.pem -out ${certDir}/${app}-csr.pem -subj "/CN=${appCN}" -config ${certDir}/${app}-req.cnf  > /dev/null 2>&1

echo "Generating ${app}'s Client Certificate"
openssl x509 -req -in ${certDir}/${app}-csr.pem -CA ${certDir}/${app}-ca.pem -CAkey ${certDir}/${app}-ca-key.pem -CAcreateserial -out ${certDir}/${app}-cert.pem -days 10 -extensions v3_req -extfile ${certDir}/${app}-req.cnf  > /dev/null 2>&1

rm ${certDir}/${app}-csr.pem ${certDir}/${app}-req.cnf
