#! /bin/bash

make undeploy

make docker-build IMG=bpalmer/scoped-operator-poc:scoped-cache

kind load docker-image bpalmer/scoped-operator-poc:scoped-cache

make deploy IMG=bpalmer/scoped-operator-poc:scoped-cache

kubectl -n scoped-memcached-operator-system get pods