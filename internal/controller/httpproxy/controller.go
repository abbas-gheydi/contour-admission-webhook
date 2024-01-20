/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/snapp-incubator/contour-admission-webhook/pkg/utils"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	finalizerString string = "snappcloud.io/httpproxy-webhook-cache"
)

//+kubebuilder:rbac:groups=projectcontour.io,resources=httpproxy,verbs=get;list;update;watch
//+kubebuilder:rbac:groups=projectcontour.io,resources=httpproxy/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// It instantiate a new ReconcilerExtended struct and start the reconciliation flow.
func (re *ReconcilerExtended) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("reconcile")

	re.logger = logger
	re.request = &req
	re.httpproxy = &contourv1.HTTPProxy{}

	result, err := re.manageLogic(ctx)

	return *result, err
}

// manageLogic manages the reconciliation logics and flow.
func (re *ReconcilerExtended) manageLogic(ctx context.Context) (*ctrl.Result, error) {
	var logicFuncs []logicFunc

	err := re.Client.Get(ctx, re.request.NamespacedName, re.httpproxy)

	if client.IgnoreNotFound(err) != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to get the httpproxy object: %w", err)
	} else if err != nil {
		// HTTPProxy object not found.
		// Return and don't requeue.
		re.logger.Info("httpproxy object not found; returned and not requeued")

		//nolint:nilerr
		return &ctrl.Result{Requeue: false}, nil
	}

	re.logger = re.logger.WithValues("namespace", re.request.Namespace)

	if utils.IsDeleted(re.httpproxy) {
		logicFuncs = append(logicFuncs,
			re.removeFinalizer)
	} else {
		logicFuncs = append(logicFuncs,
			re.addFinalizer)
	}

	// logicFuncs execution steps:
	// 1. Call each logicFunc and evaluate its result and error.
	// 2. If both the result and the error are nil, call the next logicFunc (Go to step 1).
	// 3. If the result or the error is not nil, return the result and err.
	for _, logicFunc := range logicFuncs {
		result, err := logicFunc(ctx)
		if (result != nil) || (err != nil) {
			return result, err
		}
	}

	return &ctrl.Result{Requeue: false}, nil
}

// addFinalizer adds the finalizer string to the HTTPProxy object and update it if the object's list of finalizers is updated.
func (re *ReconcilerExtended) addFinalizer(ctx context.Context) (*ctrl.Result, error) {
	err := retry.RetryOnConflict(retry.DefaultBackoff,
		func() error {
			if err := re.Client.Get(ctx, re.request.NamespacedName, re.httpproxy); err != nil {
				return err
			}

			if controllerutil.ContainsFinalizer(re.httpproxy, finalizerString) {
				return nil
			}

			controllerutil.AddFinalizer(re.httpproxy, finalizerString)

			return re.Client.Update(ctx, re.httpproxy)
		},
	)

	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to add finalizer to the httpproxy object: %w", err)
	}

	return nil, nil
}

// removeFinalizer remove the finalizer string from the HTTPProxy object and update it if the object's list of finalizers is updated.
func (re *ReconcilerExtended) removeFinalizer(ctx context.Context) (*ctrl.Result, error) {
	err := retry.RetryOnConflict(retry.DefaultBackoff,
		func() error {
			if err := re.Client.Get(ctx, re.request.NamespacedName, re.httpproxy); err != nil {
				return err
			}
			if !controllerutil.ContainsFinalizer(re.httpproxy, finalizerString) {
				return nil
			}
			controllerutil.RemoveFinalizer(re.httpproxy, finalizerString)

			return re.Client.Update(ctx, re.httpproxy)
		},
	)

	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to remove finalizer from the httpproxy object: %w", err)
	}

	return nil, nil
}
