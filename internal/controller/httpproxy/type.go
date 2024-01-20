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

	"github.com/go-logr/logr"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/snapp-incubator/contour-admission-webhook/internal/cache"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type eventType string

const (
	createEvent  eventType = "CREATE"
	updateEvent  eventType = "UPDATE"
	deleteEvent  eventType = "DELETE"
	genericEvent eventType = "GENERIC"
)

// logicFunc is a function definition representing separated reconciliation logic.
type logicFunc func(context.Context) (*ctrl.Result, error)

// customEventHandlerFunc is a function definition representing different handlers.
type customEventHandlerFunc func(context.Context, client.Object, client.Object, eventType) []ctrl.Request

type ReconcilerExtended struct {
	cache *cache.Cache
	client.Client
	// eventType    eventType
	// httpproxyNew *contourv1.HTTPProxy
	// httpproxyOld *contourv1.HTTPProxy
	httpproxy *contourv1.HTTPProxy
	logger    logr.Logger
	request   *reconcile.Request
	scheme    *runtime.Scheme
}

// customEventHandler is a struct that implements the handler.EventHandler interface.
// It is specifically implemented to manage certain types of events using the 'customHandlerFunc' function type.
// This is essential because it deals with 'event types' crucial in the 'cache' handling logic and,
// such event types are abstracted away by standard handlers.
type customEventHandler struct {
	handler customEventHandlerFunc
}
