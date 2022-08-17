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
	Scheme *runtime.Scheme
	Cache  cache.Cache
	Ctrl   controller.Controller
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

	// Check if the Memcached instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isMemcachedMarkedToBeDeleted := memcached.GetDeletionTimestamp() != nil
	if isMemcachedMarkedToBeDeleted {
		log.Info("Memcached is being deleted")
		if controllerutil.ContainsFinalizer(memcached, memcachedFinalizer) {
			// Run finalization logic for memcachedFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeMemcached(log, memcached); err != nil {
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
					return ctrl.Result{}, err
				}

			} else {
				log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			}
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
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
			return ctrl.Result{}, err
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
		return ctrl.Result{}, err
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
				log.Error(err, "Failed to create cache")
				return err
			}

			err = sc.AddResourceCache(ctx, memcached, memcachedCache)
			if err != nil {
				log.Error(err, "failed to add resource cache")
				return err
			}

			depInf, err := memcachedCache.GetInformerForKind(context.TODO(), appsv1.SchemeGroupVersion.WithKind("Deployment"))
			if err != nil {
				return err
			}

			depi := depInf.(toolscache.SharedIndexInformer)
			depi.SetWatchErrorHandler(
				func(r *toolscache.Reflector, err error) {
					//do nothing
				},
			)

			// Create a watch on Deployments
			err = r.Ctrl.Watch(&source.Informer{Informer: depi}, &handler.EnqueueRequestForOwner{
				OwnerType:    &cachev1alpha1.Memcached{},
				IsController: true,
			})
			if err != nil {
				log.Error(err, "failed to create watch for deployment")
				return err
			}

			podInf, err := memcachedCache.GetInformerForKind(context.TODO(), corev1.SchemeGroupVersion.WithKind("Pod"))
			if err != nil {
				return err
			}

			podi := podInf.(toolscache.SharedIndexInformer)
			podi.SetWatchErrorHandler(
				func(r *toolscache.Reflector, err error) {
					//do nothing
				},
			)

			err = r.Ctrl.Watch(&source.Informer{Informer: podi}, &handler.EnqueueRequestForOwner{
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

func (r *MemcachedReconciler) finalizeMemcached(log logr.Logger, m *cachev1alpha1.Memcached) error {
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
	return nil
}
