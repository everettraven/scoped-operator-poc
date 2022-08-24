/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	scopecache "github.com/everettraven/scoped-cache-poc/pkg/cache"
	cachev1alpha1 "github.com/example/memcached-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

const memcachedFinalizer = "cache.example.com/finalizer"

// MemcachedReconciler reconciles a Memcached object
type MemcachedReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Cache         cache.Cache
	Ctrl          controller.Controller
	watchesCancel context.CancelFunc
	// store contexts used to start caches for a given memcached CR
	// memcachedToCtx map[types.UID]context.Context
}

//+kubebuilder:rbac:groups=cache.example.com,resources=memcacheds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.example.com,resources=memcacheds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.example.com,resources=memcacheds/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Memcached object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *MemcachedReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the Memcached instance
	memcached := &cachev1alpha1.Memcached{}
	err := r.Get(ctx, req.NamespacedName, memcached)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Memcached resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Memcached")
		return ctrl.Result{}, err
	}

	// // Check if the Memcached instance is marked to be deleted, which is
	// // indicated by the deletion timestamp being set.
	isMemcachedMarkedToBeDeleted := memcached.GetDeletionTimestamp() != nil
	if isMemcachedMarkedToBeDeleted {
		log.Info("Memcached is being deleted")
		if controllerutil.ContainsFinalizer(memcached, memcachedFinalizer) {
			// Run finalization logic for memcachedFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.removeCacheForMemcached(log, memcached); err != nil {
				return ctrl.Result{}, err
			}

			// Remove memcachedFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(memcached, memcachedFinalizer)
			err := r.Update(ctx, memcached)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(memcached, memcachedFinalizer) {
		controllerutil.AddFinalizer(memcached, memcachedFinalizer)
		err = r.Update(ctx, memcached)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create our caches & watches for Deployments and Pods
	err = r.watchesForMemcached(ctx, memcached, log)
	if err != nil {
		log.Error(err, "failed to create watches")
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	log.Info("Getting Deployment")
	err = r.Get(ctx, types.NamespacedName{Name: memcached.Name, Namespace: memcached.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := deploymentForMemcached(memcached, r.Scheme)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			if errors.IsForbidden(err) {
				log.Info("Not permitted to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
				memcached.Status.State = cachev1alpha1.MemcachedState{
					Status:  "Failed",
					Message: fmt.Sprintf("Not permitted to create new Deployment: %s", err),
				}
				err := r.Status().Update(ctx, memcached)
				if err != nil {
					log.Error(err, "Failed to update Memcached status")
					return ctrl.Result{Requeue: true}, err
				}

				// Requeue after one minute in case RBAC changes
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err

			} else {
				log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			}
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil && errors.IsForbidden(err) {
		log.Info("Not permitted to get Deployment")
		memcached.Status.State = cachev1alpha1.MemcachedState{
			Status:  "Failed",
			Message: fmt.Sprintf("Not permitted to get Deployment: %s", err),
		}
		err := r.Status().Update(ctx, memcached)
		if err != nil {
			log.Error(err, "Failed to update Memcached status")
			return ctrl.Result{Requeue: true}, err
		}

		//if we don't have the permissions we need, then we need to remove the ResourceCache that was created for this CR
		log.Info("Removing cache for memcached CR due to invalid permissions")
		err = r.removeCacheForMemcached(log, memcached)
		if err != nil {
			log.Error(err, "failed to remove cache for memcached CR")
			return ctrl.Result{}, err
		}

		// Requeue after one minute in case RBAC changes
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := memcached.Spec.Size
	if *found.Spec.Replicas != size {
		log.Info("Deployment Spec.Replicas does not match Memcached CR Spec.Size -- Updating Deployment")
		found.Spec.Replicas = &size
		err = r.Update(ctx, found)
		// In a real operator we would want to check for a Forbidden error here as well
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return ctrl.Result{}, err
		}
		// Ask to requeue after 1 minute in order to give enough time for the
		// pods be created on the cluster side and the operand be able
		// to do the next update step accurately.
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Update the Memcached status with the pod names
	// List the pods for this memcached's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(memcached.Namespace),
		client.MatchingLabels(labelsForMemcached(memcached.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		if errors.IsForbidden(err) {
			log.Info("Not permitted to list pods")
			memcached.Status.State = cachev1alpha1.MemcachedState{
				Status:  "Failed",
				Message: fmt.Sprintf("Not permitted to list pods: %s", err),
			}
			err := r.Status().Update(ctx, memcached)
			if err != nil {
				log.Error(err, "Failed to update Memcached status")
				return ctrl.Result{Requeue: true}, err
			}

			//if we don't have the permissions we need, then we need to remove the ResourceCache that was created for this CR
			log.Info("Removing cache for memcached CR due to invalid permissions")
			err = r.removeCacheForMemcached(log, memcached)
			if err != nil {
				log.Error(err, "failed to remove cache for memcached CR")
				return ctrl.Result{}, err
			}

			// Requeue after one minute in case RBAC changes
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
		log.Error(err, "Failed to list pods", "Memcached.Namespace", memcached.Namespace, "Memcached.Name", memcached.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, memcached.Status.Nodes) {
		memcached.Status.Nodes = podNames
		err := r.Status().Update(ctx, memcached)
		if err != nil {
			log.Error(err, "Failed to update Memcached status")
			return ctrl.Result{Requeue: true}, err
		}
	}

	// if we have made it here, set the state to successful
	memcached.Status.State = cachev1alpha1.MemcachedState{
		Status:  "Succeeded",
		Message: fmt.Sprintf("Deployment %s successfully created", memcached.Name),
	}
	err = r.Status().Update(ctx, memcached)
	if err != nil {
		log.Error(err, "Failed to update Memcached status")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (r *MemcachedReconciler) watchesForMemcached(ctx context.Context, memcached *cachev1alpha1.Memcached, log logr.Logger) error {
	// Create watch for the deployment with the ScopedCache
	sc, ok := r.Cache.(*scopecache.ScopedCache)
	if !ok {
		err := fmt.Errorf("cache is not of type ScopedCache")
		log.Error(err, "failed to get scoped cache")
		return err
	}
	if ok {
		createCache := false
		rCache, cacheHasNamespace := sc.GetResourceCache()[memcached.GetNamespace()]

		if cacheHasNamespace {
			if _, cacheHasResource := rCache[memcached.GetUID()]; !cacheHasResource {
				createCache = true
			}
		} else {
			createCache = true
		}

		// Only create the cache and watches for the CR if it does not already exist
		if createCache {
			ctx, cancelFunc := context.WithCancel(ctx)

			log.Info("Creating cache for memcached CR", "CR UID:", memcached.GetUID())
			cfg := ctrl.GetConfigOrDie()
			addToMapper := func(baseMapper *meta.DefaultRESTMapper) {
				baseMapper.Add(appsv1.SchemeGroupVersion.WithKind("Deployment"), meta.RESTScopeNamespace)
				baseMapper.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
			}
			mapper, err := apiutil.NewDynamicRESTMapper(cfg, apiutil.WithCustomMapper(func() (meta.RESTMapper, error) {
				basemapper := meta.NewDefaultRESTMapper(nil)
				addToMapper(basemapper)

				return basemapper, nil
			}))
			if err != nil {
				log.Error(err, "Failed to create rest mapper")
				cancelFunc()
				return err
			}

			memcachedCache, err := cache.New(
				cfg,
				cache.Options{
					Namespace: memcached.GetNamespace(),
					DefaultSelector: cache.ObjectSelector{
						Label: labels.SelectorFromSet(labels.Set{
							"memcachedLabel": "memcached-" + memcached.Name,
						}),
						Field: fields.AndSelectors(fields.SelectorFromSet(fields.Set{
							"metadata.namespace": memcached.GetNamespace(),
						})),
					},
					Mapper: mapper,
				},
			)
			if err != nil {
				log.Error(err, "failed to create cache")
				cancelFunc()
				return err
			}

			err = sc.AddResourceCache(memcached, memcachedCache)
			if err != nil {
				log.Error(err, "failed to add resource cache")
				cancelFunc()
				return err
			}

			// Get informer for deployments
			depInf, err := r.Cache.GetInformer(ctx, &appsv1.Deployment{})
			if err != nil {
				cancelFunc()
				return fmt.Errorf("encountered an error getting deployment informer: %w", err)
			}

			watchErrorHandler := func(ref *toolscache.Reflector, err error) {
				if strings.Contains(err.Error(), "is forbidden") {
					// close the context used to start a ResourceCache for the given Memcached CR
					// at the very least this *should* stop the informers associated with the ResourceCache
					cancelFunc()

					// we should also attempt to remove the resource cache for this Memcached CR
					// so the next time it is reconciled it will attempt to create a new ResourceCache and informers
					log.Info("Removing resource cache for memcached resource due to invalid permissions", "memcached", memcached)
					removeCacheErr := sc.RemoveResourceCache(memcached)
					if removeCacheErr != nil {
						log.Error(removeCacheErr, "failed to remove resource cache for memcached resource")
					}
				}
			}

			depI, _ := depInf.(*scopecache.ScopedInformer)
			err = depI.SetWatchErrorHandler(watchErrorHandler)
			if err != nil {
				log.Error(err, "failed to SetWatchErrorHandler")
				return err
			}

			// Get informer for pods
			podInf, err := r.Cache.GetInformer(ctx, &corev1.Pod{})
			if err != nil {
				return fmt.Errorf("encountered an error getting pod informer: %w", err)
			}

			podI, _ := podInf.(*scopecache.ScopedInformer)
			err = podI.SetWatchErrorHandler(watchErrorHandler)
			if err != nil {
				log.Error(err, "failed to SetWatchErrorHandler")
				return err
			}

			// informers have been configured so lets start the ResourceCache
			sc.StartResourceCache(ctx, memcached)

			// Create a watch on Deployments
			err = r.Ctrl.Watch(&source.Informer{Informer: depI}, &handler.EnqueueRequestForOwner{
				OwnerType:    &cachev1alpha1.Memcached{},
				IsController: true,
			})
			if err != nil {
				log.Error(err, "failed to create watch for deployment")
				return err
			}

			// Create a watch on Pods
			err = r.Ctrl.Watch(&source.Informer{Informer: podI}, &handler.EnqueueRequestForOwner{
				OwnerType:    &cachev1alpha1.Memcached{},
				IsController: true,
			})
			if err != nil {
				log.Error(err, "failed to create watch for pod")
				return err
			}
		}
	}

	return nil
}

func (r *MemcachedReconciler) removeCacheForMemcached(log logr.Logger, m *cachev1alpha1.Memcached) error {
	// remove the cache and informers for this CR
	sc, ok := r.Cache.(*scopecache.ScopedCache)
	if !ok {
		err := fmt.Errorf("cache is not of type ScopedCache")
		log.Error(err, "failed to get scoped cache")
		return err
	}

	log.Info("Removing ResourceCache", "CR UID:", m.GetUID(), "ResourceCache", sc.GetResourceCache()[m.GetNamespace()])
	sc.RemoveResourceCache(m)

	// Get the Resource Cache to ensure it was removed
	if _, ok := sc.GetResourceCache()[m.GetNamespace()][m.GetUID()]; !ok {
		log.Info("ResourceCache successfully removed", "CR UID:", m.GetUID(), "ResourceCache", sc.GetResourceCache()[m.GetNamespace()])
	} else {
		log.Error(fmt.Errorf("ResourceCache not removed for CR UID: %s", m.GetUID()), "ResourceCache", sc.GetResourceCache()[m.GetNamespace()])
		return fmt.Errorf("ResourceCache not removed for CR UID: %s", m.GetUID())
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MemcachedReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Memcached{}).
		Build(r)

	if err != nil {
		return err
	}

	r.Ctrl = controller

	_, r.watchesCancel = context.WithCancel(context.TODO())
	// r.memcachedToCtx = make(map[types.UID]context.Context)
	return nil
}
