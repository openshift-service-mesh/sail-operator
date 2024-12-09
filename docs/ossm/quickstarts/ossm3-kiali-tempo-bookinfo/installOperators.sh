#!/bin/bash

echo "This script installs operators from OperatorHub"

oc apply -f ./resources/subscriptions.yaml
echo "Waiting till all operators pods are ready"
until oc get pods -n openshift-operators | grep servicemesh-operator3 | grep Running; do echo "Waiting for servicemesh-operator3 to be running."; sleep 10;done
until oc get pods -n openshift-operators | grep kiali-operator | grep Running; do echo "Waiting for kiali-operator to be running."; sleep 10;done
until oc get pods -n openshift-operators | grep opentelemetry-operator | grep Running; do echo "Waiting for opentelemetry-operator to be running."; sleep 10;done
until oc get pods -n openshift-operators | grep tempo-operator | grep Running; do echo "Waiting for tempo-operator to be running."; sleep 10;done

echo "All operators were installed successfully"
oc get pods -n openshift-operators