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
	"context"
	"time"

	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	"github.com/google/uuid"
	"github.com/zongzw/f5-bigip-rest/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	// Cache    cache.Cache
	LogLevel string
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Secret{}).
		Complete(r)
}

func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	lctx := context.WithValue(ctx, utils.CtxKey_Logger, utils.NewLog().WithRequestID(uuid.New().String()).WithLevel(r.LogLevel))
	slog := utils.LogFromContext(lctx)

	var obj v1.Secret
	slog.Infof("serect event: %s", req.NamespacedName)

	if err := r.Client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		// Can not find Sercet, remove it from the local cache
		pkg.ActiveSIGs.UnsetSerect(req.NamespacedName.String())
		return ctrl.Result{}, nil
	}
	// Find Secret, add it to the local cache.
	pkg.ActiveSIGs.SetSecret(obj.DeepCopy())
	return ctrl.Result{}, nil
}
