#! /bin/bash

kind delete cluster

kind create cluster

kubectl apply -f namespace-scoping.yaml

kubectl apply -f perf-cluster-rbac.yaml

kubectl create ns allowed-one && \
kubectl create ns allowed-two && \
kubectl create ns denied

kubectl apply -f scoped-rbac.yaml