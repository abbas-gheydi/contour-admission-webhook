// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"os"
	"time"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/snapp-incubator/contour-admission-webhook/internal/cache"
	"github.com/snapp-incubator/contour-admission-webhook/internal/config"
	controller "github.com/snapp-incubator/contour-admission-webhook/internal/controller/httpproxy"
	"github.com/snapp-incubator/contour-admission-webhook/internal/webhook"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme = runtime.NewScheme()
	logger = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(contourv1.AddToScheme(scheme))
}

func main() {
	ctrl.SetLogger(zap.New())

	var configFilePath string

	flag.StringVar(&configFilePath, "config-file-path", "./config.yaml", "The config file path.")
	flag.Parse()

	logger.Info("initializing config")

	if err := config.InitializeConfig(configFilePath); err != nil {
		logger.Error(err, "error reading the config file")

		os.Exit(1)
	}

	logger.Info("initializing cache")

	cfg := config.GetConfig()

	cache := cache.NewCache(time.Duration(cfg.Cache.CleanUpIntervalSecond) * time.Second)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:         scheme,
		LeaderElection: false,
	})
	if err != nil {
		logger.Error(err, "unable to create manager")

		os.Exit(1)
	}

	reconcilerExtended := controller.NewReconcilerExtended(mgr, cache)

	if err = reconcilerExtended.SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to set up the controller with the manager", "controller", "httpproxy")

		os.Exit(1)
	}

	errChan := make(chan error)

	ctx := ctrl.SetupSignalHandler()

	go func() {
		logger.Info("starting controller(s)")

		if err := mgr.Start(ctx); err != nil {
			errChan <- err
		}

		errChan <- nil
	}()

	stopCh := make(chan struct{})

	// This call is non-blocking
	webhookStoppedCh, webhookListenerStoppedCh := webhook.NewWebhook().
		AddCache(cache).
		ShouldMutate().
		ShouldValidate().
		Setup(stopCh)

	select {
	case err := <-errChan:
		logger.Error(err, "encountered error; exited")

		os.Exit(1)

	case <-ctx.Done():
		logger.Info("shutdown signal received; stopping webhook")

		close(stopCh)

		logger.Info("waiting for webhook to finish")

		<-webhookStoppedCh

		logger.Info("all active requests have been processed")

		<-webhookListenerStoppedCh

		logger.Info("webhook http server stopped listening")

		cache.CleanUpStopChan <- true

		os.Exit(0)
	}
}
