# Scoped Cache Demo

## Demo

1. Run `setup.sh` to:
    - Delete existing KinD cluster
    - Create a new KinD cluster
    - Apply RBAC to give * permissions for Memcached resources on the cluster
    - Create namespaces `allowed-one`, `allowed-two`, `denied`
    - Apply RBAC to give * permissions for Deployment resources in the `allowed-one` and `allowed-two` namespaces
    - Apply RBAC to give `get`, `list`, `watch` permissiosn for Pod resources in the `allowed-one` and `allowed-two` namespaces

2. Run `redeploy.sh` to:
    - Remove any existing deployments of the operator from the cluster
    - Build the image for the operator
    - Load the built image to the KinD cluster
    - Deploy the operator on the cluster
    - List the pods in the `scoped-memcached-operator-system` namespace so we can easily copy the pod name for when we take a look at the pod logs

3. Get the logs by running:
```
kubectl -n scoped-memcached-operator-system logs <pod-name>
```

We should see that the operator has started successfully:
```
1.6608304693692005e+09  INFO    controller-runtime.metrics      Metrics server is starting to listen    {"addr": "127.0.0.1:8080"}
1.660830469369386e+09   INFO    setup   starting manager
1.6608304693696425e+09  INFO    Starting server {"kind": "health probe", "addr": "[::]:8081"}
1.6608304693696718e+09  INFO    Starting server {"path": "/metrics", "kind": "metrics", "addr": "127.0.0.1:8080"}
I0818 13:47:49.369663       1 leaderelection.go:248] attempting to acquire leader lease scoped-memcached-operator-system/86f835c3.example.com...
I0818 13:47:49.373329       1 leaderelection.go:258] successfully acquired lease scoped-memcached-operator-system/86f835c3.example.com
1.6608304693733532e+09  DEBUG   events  Normal  {"object": {"kind":"Lease","namespace":"scoped-memcached-operator-system","name":"86f835c3.example.com","uid":"ac82b4f7-a193-4d52-864b-315e1fc80ce1","apiVersion":"coordination.k8s.io/v1","resourceVersion":"564"}, "reason": "LeaderElection", "message": "scoped-memcached-operator-controller-manager-7b4c9bb485-7jlkh_666ab6d0-e194-43c8-8348-724608e1521e became leader"}
1.6608304693735454e+09  INFO    Starting EventSource    {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "source": "kind source: *v1alpha1.Memcached"}
1.6608304693736055e+09  INFO    Starting Controller     {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached"}
1.660830469473995e+09   INFO    Starting workers        {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "worker count": 1}
```

4. Create some `Memcached` resources in the `allowed-one` and `allowed-two` namespaces by running:
```
kubectl apply -f config/samples/cache_v1alpha1_memcached.yaml
```

5. Get the logs again:
```
kubectl -n scoped-memcached-operator-system logs <pod-name>
```

For each CR we should see that there are logs signifying that:
- A cache has been created for the `Memcached` CR
- 2 event sources (watches) have been started (one is for `Deployment`s created by the controller and one is for `Pod`s created from the `Deployment`s)
- Attempt to get a deployment
- Creation of a deployment

The logs should look similar to:
```
1.6608306977001324e+09  INFO    Creating cache for memcached CR {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-allowed-one","namespace":"allowed-one"}, "namespace": "allowed-one", "name": "memcached-sample-allowed-one", "reconcileID": "188d52ee-593c-4716-980f-b4fe500bdb6c", "CR UID:": "b2427753-f092-4e22-a633-4d39bea7a0c4"}
1.660830697700437e+09   INFO    Starting EventSource    {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "source": "informer source: 0xc0000a4640"}
1.6608306978010592e+09  INFO    Starting EventSource    {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "source": "informer source: 0xc0000a4780"}
1.6608306978010962e+09  INFO    Getting Deployment      {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-allowed-one","namespace":"allowed-one"}, "namespace": "allowed-one", "name": "memcached-sample-allowed-one", "reconcileID": "188d52ee-593c-4716-980f-b4fe500bdb6c"}
1.660830697801139e+09   INFO    Creating a new Deployment       {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-allowed-one","namespace":"allowed-one"}, "namespace": "allowed-one", "name": "memcached-sample-allowed-one", "reconcileID": "188d52ee-593c-4716-980f-b4fe500bdb6c", "Deployment.Namespace": "allowed-one", "Deployment.Name": "memcached-sample-allowed-one"}
1.6608306978104348e+09  INFO    Creating cache for memcached CR {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-allowed-two","namespace":"allowed-two"}, "namespace": "allowed-two", "name": "memcached-sample-allowed-two", "reconcileID": "9bdb2035-db34-412b-bf2f-1496df56c134", "CR UID:": "690a5864-af57-4e6a-a75a-efce861d09cf"}
1.660830697810686e+09   INFO    Starting EventSource    {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "source": "informer source: 0xc0001b0e60"}
1.6608306978107378e+09  INFO    Starting EventSource    {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "source": "informer source: 0xc0001b10e0"}
1.6608306978107502e+09  INFO    Getting Deployment      {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-allowed-two","namespace":"allowed-two"}, "namespace": "allowed-two", "name": "memcached-sample-allowed-two", "reconcileID": "9bdb2035-db34-412b-bf2f-1496df56c134"}
1.6608306978107738e+09  INFO    Creating a new Deployment       {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-allowed-two","namespace":"allowed-two"}, "namespace": "allowed-two", "name": "memcached-sample-allowed-two", "reconcileID": "9bdb2035-db34-412b-bf2f-1496df56c134", "Deployment.Namespace": "allowed-two", "Deployment.Name": "memcached-sample-allowed-two"}
```

As the deployments are spun up and reconciled, the deployment may be modified. This operator sets ownership on deployments and will reconcile the parent `Memcached` CR whenever a child deployment is modified. You may see a chunk of logs similar to (example truncated to only a couple logs for brevity):
```
1.6608307072214928e+09  INFO    Getting Deployment      {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-allowed-one","namespace":"allowed-one"}, "namespace": "allowed-one", "name": "memcached-sample-allowed-one", "reconcileID": "fa336c14-f699-4f70-89d0-37631770441f"}
1.660830707233768e+09   INFO    Getting Deployment      {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-allowed-two","namespace":"allowed-two"}, "namespace": "allowed-two", "name": "memcached-sample-allowed-two", "reconcileID": "644fd96b-346b-47b8-8c36-784e1741bbbb"}
```

6. Check the namespaces to see that the proper deployments are created:
```
kubectl -n allowed-one get deploy
```
Output should look like:
```
NAME                           READY   UP-TO-DATE   AVAILABLE   AGE
memcached-sample-allowed-one   2/2     2            2           13m
```

```
kubectl -n allowed-two get deploy
```
Output should look like:
```
NAME                           READY   UP-TO-DATE   AVAILABLE   AGE
memcached-sample-allowed-two   3/3     3            3           14m
```

7. Let's see what happens when we create a `Memcached` CR in a namespace that the operator does not have proper permissions in:

Create a `Memcached` CR in the namespace `denied` by running:
```
cat << EOF | kubectl apply -f -
apiVersion: cache.example.com/v1alpha1
kind: Memcached
metadata:
  name: memcached-sample-denied
  namespace: denied
spec:
  size: 1
EOF
```

Check the logs, we should see:
```
1.6608487955810938e+09	INFO	Creating cache for memcached CR	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "865240aa-1eac-48d0-9a64-56c2eec66b88", "CR UID:": "b49142b4-cc50-4465-969c-7257049247b6"}
1.660848795581366e+09	INFO	Starting EventSource	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "source": {}}
1.6608487955813868e+09	INFO	Starting EventSource	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "source": {}}
1.660848795581394e+09	INFO	Getting Deployment	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "865240aa-1eac-48d0-9a64-56c2eec66b88"}
1.6608487971761699e+09	INFO	Not permitted to get Deployment	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "865240aa-1eac-48d0-9a64-56c2eec66b88"}
1.6612814011258633e+09  INFO    Removing cache for memcached CR due to invalid permissions      {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "ba671951-e2da-4b2b-87d9-9b0667f0c608"}
1.6612814011259036e+09  INFO    Removing ResourceCache  {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "ba671951-e2da-4b2b-87d9-9b0667f0c608", "CR UID:": "9f60a7b1-dee6-4b6c-a729-420ec651c0dc", "ResourceCache": {"9f60a7b1-dee6-4b6c-a729-420ec651c0dc":{"Scheme":{}}}}
1.6612814011259542e+09  INFO    ResourceCache successfully removed      {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "ba671951-e2da-4b2b-87d9-9b0667f0c608", "CR UID:": "9f60a7b1-dee6-4b6c-a729-420ec651c0dc", "ResourceCache": {}}
```
We can see we are also removing any caches that have been created for this `Memcached` CR to prevent unnecessary informers from hanging around.

Checking the `Memcached` CR with `kubectl -n denied describe memcached` we can see the status:
```
Status:
  State:
    Message:  Not permitted to get Deployment: deployments.apps "memcached-sample-denied" is forbidden: Not permitted based on RBAC
    Status:   Failed
```

8. Update the RBAC to give permissions to the denied namespace by running:
```
cat << EOF | kubectl apply -f - 
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: op-rolebinding-default
  namespace: denied
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: scoped-operator-needs
subjects:
- kind: ServiceAccount
  name: scoped-memcached-operator-controller-manager
  namespace: scoped-memcached-operator-system
EOF
```

After a little bit of time we should see in the logs:
```
1.66085439100725e+09	INFO	Creating a new Deployment	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "8bfc654a-e372-47c8-9be5-2cf89f654c34", "Deployment.Namespace": "denied", "Deployment.Name": "memcached-sample-denied"}
1.66085439102921e+09	INFO	Getting Deployment	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "0bbc7b20-f392-45dc-a210-0d10ec58ff34"}
1.660854392647686e+09	INFO	Getting Deployment	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "610f80fc-9d7e-46b1-ac25-5e5286fa97d2"}
```

We can see in the `Memcached` CR status that it has been successfully reconciled:
```
Status:
  Nodes:
    memcached-sample-denied-7685b99f49-tv2b8
  State:
    Message:  Deployment memcached-sample-denied successfully created
    Status:   Succeeded
```

We can also see that the deployment is up and running by running `kubectl -n denied get deploy`:
```
NAME                      READY   UP-TO-DATE   AVAILABLE   AGE
memcached-sample-denied   1/1     1            1           71s
```

9. Now let's restrict access again by deleting the RBAC we applied to give permissions in the `denied` namespace:
```
kubectl -n denied delete rolebinding op-rolebinding-default
```

This change won't affect the existing `Memcached` CR since it has already been reconciled, but if we edit the existing `Memcached` CR or create a new one in the `denied` namespace we will see these logs start to pop up again:
```
1.6608546666716454e+09	INFO	Getting Deployment	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "b88c4f4f-4885-4bd4-a706-dc6b888dbca7"}
1.6608546666883416e+09	INFO	Not permitted to get Deployment	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "b88c4f4f-4885-4bd4-a706-dc6b888dbca7"}
```

The `Memcached` CR status will again look like:
```
Status:
  Nodes:
    memcached-sample-denied-7685b99f49-tv2b8
  State:
    Message:  Not permitted to get Deployment: deployments.apps "memcached-sample-denied" is forbidden: Not permitted based on RBAC
    Status:   Failed
```

In this example I edited the existing `Memcached` CR to kick off the reconciliation loop which is why the `Status.Nodes` field is still populated.

Another thing to note in this case, if there is no reason for the reconciliation loop to run in the `denied` namespace the existing watches won't be cleaned up. Eventually the watches will attempt to refresh and they will encounter a `WatchError` due to permissions having been revoked. If not handled properly this will cause the Operator to enter a blocking loop where it continuously attempts to reconnect the watch. 

In this Operator, when creating informers we inject our own `WatchErrorHandler` that will close the channel used by the informers to stop them. We then remove the ResourceCache that did not have the proper permissions so that when we reconcile a CR in that namespace again we can attempt to recreate the informers in the event RBAC has changed. This handling of the `WatchError` prevents the blocking loop of continuously attempting to reconnect the watch.

In the Operator logs, this process looks like:
```
W0823 19:34:02.520559       1 reflector.go:324] pkg/mod/k8s.io/client-go@v0.24.3/tools/cache/reflector.go:167: failed to list *v1.Deployment: deployments.apps is forbidden: User "system:serviceaccount:scoped-memcached-operator-system:scoped-memcached-operator-controller-manager" cannot list resource "deployments" in API group "apps" in the namespace "denied"
1.661283242520624e+09   INFO    Removing resource cache for memcached resource due to invalid permissions     {"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "967b9ba2-14f8-4a1b-bc14-a2802940d4a4", "memcached": {"apiVersion": "cache.example.com/v1alpha1", "kind": "Memcached", "namespace": "denied", "name": "memcached-sample-denied"}}
```


10. Now lets delete the `Memcached` CR from the `denied` namespace entirely by running:
```
kubectl -n denied delete memcached memcached-sample-denied
```

Because the operator utilizes finalizers, our resource should not be deleted until the finalizer is removed. As part of the finalizer logic, we remove the cache for the `Memcached` CR that is being deleted. We should see in the logs:
```
1.6608559969812284e+09	INFO	Memcached is being deleted	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "a186a050-8ad7-4c92-855c-de59d7b371ea"}
1.6608559969812474e+09	INFO	Removing ResourceCache	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "a186a050-8ad7-4c92-855c-de59d7b371ea", "CR UID:": "eda9cac4-c3c6-4da1-b920-f374748d40cb", "ResourceCache": {"eda9cac4-c3c6-4da1-b920-f374748d40cb":{"Scheme":{}}}}
1.6608559969813137e+09	INFO	ResourceCache successfully removed	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "a186a050-8ad7-4c92-855c-de59d7b371ea", "CR UID:": "eda9cac4-c3c6-4da1-b920-f374748d40cb", "ResourceCache": {}}
1.660855996986491e+09	INFO	Memcached resource not found. Ignoring since object must be deleted	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "aa1eef8d-90ac-428c-b434-d76abdcf167b"}
1.6608560039957695e+09	INFO	Memcached resource not found. Ignoring since object must be deleted	{"controller": "memcached", "controllerGroup": "cache.example.com", "controllerKind": "Memcached", "memcached": {"name":"memcached-sample-denied","namespace":"denied"}, "namespace": "denied", "name": "memcached-sample-denied", "reconcileID": "f51f0c62-0237-468b-8e55-7a6d03a0d400"}
```