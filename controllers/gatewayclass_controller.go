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
	"fmt"
	"time"

	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	"github.com/google/uuid"
	"github.com/zongzw/f5-bigip-rest/deployer"
	"github.com/zongzw/f5-bigip-rest/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayClassReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	LogLevel string
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Adc object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *GatewayClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lctx := context.WithValue(ctx, utils.CtxKey_Logger, utils.NewLog().WithRequestID(uuid.New().String()).WithLevel(r.LogLevel))
	slog := utils.LogFromContext(lctx)
	if req.Namespace != "" {
		return ctrl.Result{}, fmt.Errorf("gateway class namespace must be ''")
	}

	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	var obj gatewayv1beta1.GatewayClass
	slog.Debugf("handling gatewayclass " + req.Name)
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			defer pkg.ActiveSIGs.UnsetGatewayClass(req.Name)
			return handleDeletingGatewayClass(lctx, req)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		ngwc := obj.DeepCopy()

		ngwc.Status.Conditions = []metav1.Condition{
			{
				Type:               "Accepted",
				Status:             metav1.ConditionTrue,
				Reason:             string(gatewayv1beta1.GatewayClassReasonAccepted),
				Message:            "Accepted message",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}

		// TODO:
		// 1.671457016981118e+09	DEBUG	handling gatewayclass bigip	{"controller": "gatewayclass", "controllerGroup": "gateway.networking.k8s.io", "controllerKind": "GatewayClass", "GatewayClass": {"name":"bigip"}, "namespace": "", "name": "bigip", "reconcileID": "bcd01fdd-8dfc-4be9-b3ab-c3d3a3fdb67e"}
		// 1.6714570170091689e+09	ERROR	unable to update status	{"controller": "gatewayclass", "controllerGroup": "gateway.networking.k8s.io", "controllerKind": "GatewayClass", "GatewayClass": {"name":"bigip"}, "namespace": "", "name": "bigip", "reconcileID": "bcd01fdd-8dfc-4be9-b3ab-c3d3a3fdb67e", "error": "Operation cannot be fulfilled on gatewayclasses.gateway.networking.k8s.io \"bigip\": the object has been modified; please apply your changes to the latest version and try again"}
		// if err := r.Status().Update(ctx, ngwc); err != nil {
		// 	slog.Errorf("unable to update status: %s", err.Error())
		// 	return ctrl.Result{}, err
		// } else {
		// 	slog.Debugf("status updated")
		// }

		// upsert gatewayclass
		defer pkg.ActiveSIGs.SetGatewayClass(&obj)
		return handleUpsertingGatewayClass(lctx, &obj)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1beta1.GatewayClass{}).
		Complete(r)
}

func handleDeletingGatewayClass(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	slog := utils.LogFromContext(ctx)
	slog.Debugf("deleting gatewayclass " + req.Name)

	gwc := pkg.ActiveSIGs.GetGatewayClass(req.Name)
	if gwc == nil {
		return ctrl.Result{}, nil
	}
	ocfgs := map[string]interface{}{}
	// ocfgs, ncfgs := map[string]interface{}{}, map[string]interface{}{}
	opcfgs, npcfgs := map[string]interface{}{}, map[string]interface{}{}
	var err error

	gws := pkg.ActiveSIGs.AttachedGateways(gwc)
	if ocfgs, err = pkg.ParseGatewayRelatedForClass(gwc.Name, gws); err != nil {
		return ctrl.Result{}, err
	}
	if opcfgs, err = pkg.ParseServicesRelatedForAll(); err != nil {
		return ctrl.Result{}, err
	}

	pkg.ActiveSIGs.UnsetGatewayClass(req.Name)

	if npcfgs, err = pkg.ParseServicesRelatedForAll(); err != nil {
		return ctrl.Result{}, err
	}

	dctx := context.WithValue(ctx, deployer.CtxKey_DeletePartition, "yes")
	pkg.PendingDeploys <- deployer.DeployRequest{
		Meta:      fmt.Sprintf("clearing gateways for gatewayclass '%s'", req.Name),
		From:      &ocfgs,
		To:        nil,
		Partition: req.Name,
		Context:   dctx,
	}

	pkg.PendingDeploys <- deployer.DeployRequest{
		Meta:      fmt.Sprintf("updating services for gatewayclass '%s'", req.Name),
		From:      &opcfgs,
		To:        &npcfgs,
		Partition: "cis-c-tenant",
		Context:   ctx,
	}

	return ctrl.Result{}, nil
}

func handleUpsertingGatewayClass(ctx context.Context, obj *gatewayv1beta1.GatewayClass) (ctrl.Result, error) {
	slog := utils.LogFromContext(ctx)

	reqn := utils.Keyname(obj.Namespace, obj.Name)
	slog.Debugf("upserting gatewayclass " + reqn)
	ngwc := obj.DeepCopy()

	if ngwc.Spec.ControllerName != gatewayv1beta1.GatewayController(pkg.ActiveSIGs.ControllerName) {
		slog.Debugf("ignore this gwc " + reqn + " as its controllerName does not match " + pkg.ActiveSIGs.ControllerName)
		return ctrl.Result{}, nil
	}

	ocfgs, ncfgs := map[string]interface{}{}, map[string]interface{}{}
	opcfgs, npcfgs := map[string]interface{}{}, map[string]interface{}{}
	var err error

	if ogwc := pkg.ActiveSIGs.GetGatewayClass(reqn); ogwc != nil {
		gws := pkg.ActiveSIGs.AttachedGateways(ogwc)
		if ocfgs, err = pkg.ParseGatewayRelatedForClass(ogwc.Name, gws); err != nil {
			return ctrl.Result{}, err
		}
	}
	if opcfgs, err = pkg.ParseServicesRelatedForAll(); err != nil {
		return ctrl.Result{}, err
	}

	pkg.ActiveSIGs.SetGatewayClass(ngwc)

	if npcfgs, err = pkg.ParseServicesRelatedForAll(); err != nil {
		return ctrl.Result{}, err
	}

	gws := pkg.ActiveSIGs.AttachedGateways(ngwc)
	if ncfgs, err = pkg.ParseGatewayRelatedForClass(ngwc.Name, gws); err != nil {
		return ctrl.Result{}, err
	}

	pkg.PendingDeploys <- deployer.DeployRequest{
		Meta:      fmt.Sprintf("refreshing services for gatewayclass '%s'", reqn),
		From:      &opcfgs,
		To:        &npcfgs,
		Partition: "cis-c-tenant",
		Context:   ctx,
	}

	cctx := context.WithValue(ctx, deployer.CtxKey_CreatePartition, "yes")
	pkg.PendingDeploys <- deployer.DeployRequest{
		Meta:      fmt.Sprintf("refreshing gateways for gatewayclass '%s'", reqn),
		From:      &ocfgs,
		To:        &ncfgs,
		Partition: reqn,
		Context:   cctx,
	}

	return ctrl.Result{}, nil
}
