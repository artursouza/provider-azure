/*
Copyright 2020 The Crossplane Authors.

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

package cosmosdb

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-azure/apis/database/v1beta1"
	"github.com/crossplane/provider-azure/apis/v1alpha3"
	"github.com/crossplane/provider-azure/pkg/clients/database/cosmosdb"
)

// Error strings
const (
	errProviderSecretNil  = "provider does not have a secret reference"
	errNotNoSQLAccount    = "managed resource is not a Database Account"
	errCreateNoSQLAccount = "cannot create Database Account"
	errGetNoSQLAccount    = "cannot get Database Account"
	errDeleteNoSQLAccount = "cannot delete Database Account"
)

// Setup adds a controller that reconciles NoSQLAccount.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1beta1.CosmosDBAccountGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.CosmosDBAccount{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.CosmosDBAccountGroupVersionKind),
			managed.WithConnectionPublishers(),
			managed.WithExternalConnecter(&connecter{kube: mgr.GetClient(), newClientFn: cosmosdb.NewDatabaseAccountClient}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connecter struct {
	kube        client.Client
	newClientFn func(creds []byte) (cosmosdb.AccountClient, error)
}

func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	r, ok := mg.(*v1beta1.CosmosDBAccount)
	if !ok {
		return nil, errors.New(errNotNoSQLAccount)
	}

	p := &v1alpha3.Provider{}
	n := meta.NamespacedNameOf(r.Spec.ProviderReference)
	if err := c.kube.Get(ctx, n, p); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	if p.GetCredentialsSecretReference() == nil {
		return nil, errors.New(errProviderSecretNil)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Spec.CredentialsSecretRef.Namespace, Name: p.Spec.CredentialsSecretRef.Name}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider secret %s", n)
	}

	client, err := c.newClientFn(s.Data[p.Spec.CredentialsSecretRef.Key])
	return &external{client: client}, errors.Wrap(err, "cannot create new Azure Database Account client")
}

// external is a createsyncdeleter using the Azure API.
type external struct {
	client cosmosdb.AccountClient
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	r, ok := mg.(*v1beta1.CosmosDBAccount)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotNoSQLAccount)
	}

	res, err := e.client.CheckNameExists(ctx, meta.GetExternalName(r))
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetNoSQLAccount)
	}

	if res.Response.StatusCode == http.StatusNotFound {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	account, err := e.client.Get(ctx, r.Spec.ForProvider.ResourceGroupName, meta.GetExternalName(r))
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetNoSQLAccount)
	}
	cosmosdb.UpdateCosmosDBAccountObservation(&r.Status, account)

	switch r.Status.AtProvider.State {
	case "Succeeded":
		r.SetConditions(runtimev1alpha1.Available())
		resource.SetBindable(r)
	default:
		r.SetConditions(runtimev1alpha1.Unavailable())
	}

	resourceUpToDate := cosmosdb.CheckEqualDatabaseProperties(r.Spec.ForProvider.Properties, r.Status.AtProvider.Properties)
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: resourceUpToDate}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	r, ok := mg.(*v1beta1.CosmosDBAccount)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotNoSQLAccount)
	}

	r.Status.SetConditions(runtimev1alpha1.Creating())
	_, err := e.client.CreateOrUpdate(ctx,
		r.Spec.ForProvider.ResourceGroupName,
		meta.GetExternalName(r),
		cosmosdb.ToDatabaseAccountCreate(&r.Spec))
	// TODO(artursouza): handle secrets.
	return managed.ExternalCreation{}, errors.Wrap(err, errCreateNoSQLAccount)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, err := e.Create(ctx, mg)
	return managed.ExternalUpdate{}, err
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	r, ok := mg.(*v1beta1.CosmosDBAccount)
	if !ok {
		return errors.New(errNotNoSQLAccount)
	}

	r.Status.SetConditions(runtimev1alpha1.Deleting())
	_, err := e.client.Delete(ctx, r.Spec.ForProvider.ResourceGroupName, meta.GetExternalName(r))
	return errors.Wrap(err, errDeleteNoSQLAccount)
}
