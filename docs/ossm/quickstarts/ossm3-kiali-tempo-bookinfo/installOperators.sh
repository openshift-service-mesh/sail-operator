#!/bin/bash

echo "This script installs operators from OperatorHub"

oc apply -f ./resources/subscriptions.yaml
# wait until Install plan appears
timeout --foreground 3m bash -c 'until [[ "$(oc get ip -n openshift-operators)" ]];do echo \"Waiting for install plan is created\";sleep 5;done'

IP_NAME=$(oc get ip -n openshift-operators -o=jsonpath='{.items[?(@.spec.approved==false)].metadata.name}')
echo "Approving install plan"
oc patch installplan ${IP_NAME} -n openshift-operators --type merge --patch '{"spec":{"approved":true}}'
sleep 10s

echo "Waiting till all operators pods are ready"
oc wait --for=condition=Ready pods --all -n openshift-operators --timeout 240s
oc get pods -n openshift-operators

echo "All operators were installed successfully"
