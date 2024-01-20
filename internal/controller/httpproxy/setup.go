package controller

import (
	"context"
	"errors"
	"fmt"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/snapp-incubator/contour-admission-webhook/internal/cache"
	"github.com/snapp-incubator/contour-admission-webhook/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ handler.EventHandler = &customEventHandler{}

func newCustomEventHandler(handler customEventHandlerFunc) handler.EventHandler {
	return &customEventHandler{
		handler: handler,
	}
}

func (h *customEventHandler) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	reqs := h.handler(ctx, evt.Object, nil, createEvent)

	for _, req := range reqs {
		q.Add(req)
	}
}

func (h *customEventHandler) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	reqs := h.handler(ctx, evt.ObjectNew, evt.ObjectOld, updateEvent)

	for _, req := range reqs {
		q.Add(req)
	}
}

func (h *customEventHandler) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	reqs := h.handler(ctx, nil, evt.Object, deleteEvent)

	for _, req := range reqs {
		q.Add(req)
	}
}

func (h *customEventHandler) Generic(_ context.Context, _ event.GenericEvent, _ workqueue.RateLimitingInterface) {
	// no implementation yet
}

//nolint:varnamelen
func (re *ReconcilerExtended) httpproxyEventHandler(ctx context.Context, objNew client.Object, objOld client.Object, et eventType) []ctrl.Request {
	newHttpproxy, ok := objNew.(*contourv1.HTTPProxy)
	if objNew != nil && !ok {
		return []ctrl.Request{}
	}

	oldHttpproxy, ok := objOld.(*contourv1.HTTPProxy)
	if objOld != nil && !ok {
		return []ctrl.Request{}
	}

	logger := log.FromContext(ctx).WithName("httpproxy event handler").WithValues("event", et)

	reqs := make([]ctrl.Request, 1)

	switch et {
	case createEvent:
		reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: newHttpproxy.GetNamespace(), Name: newHttpproxy.GetName()}})

		if newHttpproxy.Spec.VirtualHost == nil {
			break
		}

		ingressClassName := utils.GetIngressClassName(newHttpproxy)

		if !utils.ValidateIngressClassName(ingressClassName) {
			logger.Info("httpproxy ingressClassName is not valid: neither cached fqdn nor queued object for reconciliation")

			return []ctrl.Request{}
		}

		// The FQDN is always set and validated against pattern "^(\\*\\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$", so
		// checking for zero value is not required as it's handled in the Kube API server before persisting in the storage.
		fqdn := newHttpproxy.Spec.VirtualHost.Fqdn

		cacheKey := utils.GenerateCacheKey(ingressClassName, fqdn)

		if re.cache.KeyExists(cacheKey) {
			isKeyPersisted := re.cache.IsKeyPersisted(cacheKey)
			if isKeyPersisted != nil && *isKeyPersisted {
				errMsg := fmt.Sprintf("fqdn '%s' is used in multiple httpproxies",
					fqdn)

				err := errors.New(errMsg)

				logger.Error(err, "fqdn uniqueness is compromised")

				break
			}
		}

		// Add the entry to the cache with persistence.
		re.cache.Set(cacheKey,
			&types.NamespacedName{Namespace: newHttpproxy.GetNamespace(), Name: newHttpproxy.GetName()},
			0,
		)

	case updateEvent:
		reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: newHttpproxy.GetNamespace(), Name: newHttpproxy.GetName()}})

		if newHttpproxy.Spec.VirtualHost == nil && oldHttpproxy.Spec.VirtualHost == nil {
			break
		}

		oldIngressClassName := utils.GetIngressClassName(oldHttpproxy)

		if newHttpproxy.Spec.VirtualHost == nil && oldHttpproxy.Spec.VirtualHost != nil {
			// The FQDN is always set and validated against pattern "^(\\*\\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$", so
			// checking for zero value is not required as it's handled in the Kube API server before persisting in the storage.
			oldFqdn := oldHttpproxy.Spec.VirtualHost.Fqdn

			cacheKey := utils.GenerateCacheKey(oldIngressClassName, oldFqdn)

			re.cache.Delete(cacheKey)

			break
		}

		newIngressClassName := utils.GetIngressClassName(newHttpproxy)

		if newHttpproxy.Spec.VirtualHost != nil && oldHttpproxy.Spec.VirtualHost == nil {
			// The FQDN is always set and validated against pattern "^(\\*\\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$", so
			// checking for zero value is not required as it's handled in the Kube API server before persisting in the storage.
			newFqdn := newHttpproxy.Spec.VirtualHost.Fqdn

			cacheKey := utils.GenerateCacheKey(newIngressClassName, newFqdn)

			// Add the entry to the cache with persistence.
			re.cache.Set(cacheKey,
				&types.NamespacedName{Namespace: newHttpproxy.GetNamespace(), Name: newHttpproxy.GetName()},
				0,
			)

			break
		}

		// The FQDN is always set and validated against pattern "^(\\*\\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$", so
		// checking for zero value is not required as it's handled in the Kube API server before persisting in the storage.
		newFqdn := newHttpproxy.Spec.VirtualHost.Fqdn
		oldFqdn := oldHttpproxy.Spec.VirtualHost.Fqdn

		if newFqdn != oldFqdn || newIngressClassName != oldIngressClassName {
			newCacheKey := utils.GenerateCacheKey(newIngressClassName, newFqdn)
			oldCacheKey := utils.GenerateCacheKey(oldIngressClassName, oldFqdn)

			re.cache.Set(newCacheKey,
				&types.NamespacedName{Namespace: newHttpproxy.GetNamespace(), Name: newHttpproxy.GetName()},
				0,
			)

			re.cache.Delete(oldCacheKey)
		}

	case deleteEvent:
		reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: oldHttpproxy.GetNamespace(), Name: oldHttpproxy.GetName()}})

		if oldHttpproxy.Spec.VirtualHost == nil {
			break
		}

		ingressClassName := utils.GetIngressClassName(oldHttpproxy)

		// The FQDN is always set and validated against pattern "^(\\*\\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$", so
		// checking for zero value is not required as it's handled in the Kube API server before persisting in the storage.
		fqdn := oldHttpproxy.Spec.VirtualHost.Fqdn

		cacheKey := utils.GenerateCacheKey(ingressClassName, fqdn)

		// Remove the entry from the cache.
		re.cache.Delete(cacheKey)
	}

	return reqs
}

// NewReconcilerExtended instantiate a new ReconcilerExtended struct and returns it.
func NewReconcilerExtended(mgr manager.Manager, cache *cache.Cache) *ReconcilerExtended {
	return &ReconcilerExtended{
		cache:  cache,
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// SetupWithManager sets up the controller with the manager.
func (re *ReconcilerExtended) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// This watch EventHandler responds to create / delete / update events by *reconciling the object* ( equivalent of calling For(&contourv1.HTTPProxy{}) )
		// in addition to handling cache update flow.
		Watches(&contourv1.HTTPProxy{}, newCustomEventHandler(re.httpproxyEventHandler)).
		Named("httpproxy").
		Complete(re)
}
