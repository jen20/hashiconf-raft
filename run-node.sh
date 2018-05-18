#!/usr/bin/env bash

set -o errexit
set -o pipefail

node_number=$1

function usage() {
    echo "Usage:"
    echo "   run-node.sh <node number (1-5)>"
}

if [ -z ${node_number} ] ; then
    usage
    exit 1
fi

if ! [[ "${node_number}" =~ ^[1-5]$ ]] ; then
    usage
    exit 1
fi

node_raft_port=$((6999 + $node_number))
node_http_port=$((7999 + $node_number))
data_directory="./node${node_number}"

args="--data-dir=${data_directory}"
args="${args} --raft-port=${node_raft_port}"
args="${args} --http-port=${node_http_port}"

if [ "${node_number}" == 1 ] ; then
    args="${args} --bootstrap true"
else
    args="${args} --join=127.0.0.1:8000"
fi

./hashiconf-raft ${args}

