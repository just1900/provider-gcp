/*
Copyright 2019 The Crossplane Authors.

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

package storage

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/api/storage/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-gcp/apis/storage/v1alpha1"
	gcp "github.com/crossplane/provider-gcp/pkg/clients"
	"github.com/crossplane/provider-gcp/pkg/clients/bucketpolicy"
)

const (
	// https://cloud.google.com/iam/docs/reference/rest/v1/Policy
	// Specifies the format of the policy.
	// Any operation that affects conditional role bindings must specify version 3.
	// Our CR supports conditional role bindings.
	policyVersion = 3
)
const (
	errNotBucketPolicy = "managed resource is not a GCP BucketPolicy"
	errCheckUpToDate   = "cannot determine if BucketPolicy instance is up to date"
	errGetPolicy       = "cannot get GCP BucketPolicy object via Storage API"
	errSetPolicy       = "cannot set GCP BucketPolicy object via Storage API"
)

// SetupBucketPolicy adds a controller that reconciles BucketPolicys.
func SetupBucketPolicy(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.BucketPolicyGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.BucketPolicy{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.BucketPolicyGroupVersionKind),
			managed.WithExternalConnecter(&bucketPolicyConnecter{client: mgr.GetClient()}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type bucketPolicyConnecter struct {
	client client.Client
}

// Connect sets up iam client using credentials from the provider
func (c *bucketPolicyConnecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	_, opts, err := gcp.GetAuthInfo(ctx, c.client, mg)
	if err != nil {
		return nil, err
	}
	s, err := storage.NewService(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}
	return &bucketPolicyExternal{kube: c.client, bucketpolicy: storage.NewBucketsService(s)}, nil
}

type bucketPolicyExternal struct {
	kube         client.Client
	bucketpolicy bucketpolicy.Client
}

func (e *bucketPolicyExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.BucketPolicy)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotBucketPolicy)
	}

	instance, err := e.bucketpolicy.GetIamPolicy(gcp.StringValue(cr.Spec.ForProvider.Bucket)).OptionsRequestedPolicyVersion(policyVersion).Context(ctx).Do()
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(resource.Ignore(gcp.IsErrorNotFound, err), errGetPolicy)
	}
	// Empty policy
	if bucketpolicy.IsEmpty(instance) {
		return managed.ExternalObservation{}, nil
	}

	cr.Status.SetConditions(xpv1.Available())

	upToDate, err := bucketpolicy.IsUpToDate(&cr.Spec.ForProvider, instance)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errCheckUpToDate)
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *bucketPolicyExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.BucketPolicy)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotBucketPolicy)
	}
	cr.SetConditions(xpv1.Creating())
	instance := &storage.Policy{}
	bucketpolicy.GenerateBucketPolicyInstance(cr.Spec.ForProvider, instance)

	if _, err := e.bucketpolicy.SetIamPolicy(gcp.StringValue(cr.Spec.ForProvider.Bucket), instance).
		Context(ctx).Do(); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errSetPolicy)
	}

	return managed.ExternalCreation{}, nil
}

func (e *bucketPolicyExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.BucketPolicy)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotBucketPolicy)
	}
	instance, err := e.bucketpolicy.GetIamPolicy(gcp.StringValue(cr.Spec.ForProvider.Bucket)).OptionsRequestedPolicyVersion(policyVersion).Context(ctx).Do()
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetPolicy)
	}

	u, err := bucketpolicy.IsUpToDate(&cr.Spec.ForProvider, instance)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errCheckUpToDate)
	}
	if u {
		return managed.ExternalUpdate{}, nil
	}

	bucketpolicy.GenerateBucketPolicyInstance(cr.Spec.ForProvider, instance)
	if _, err := e.bucketpolicy.SetIamPolicy(gcp.StringValue(cr.Spec.ForProvider.Bucket), instance).
		Context(ctx).Do(); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errSetPolicy)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *bucketPolicyExternal) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.BucketPolicy)
	if !ok {
		return errors.New(errNotBucketPolicy)
	}
	if _, err := e.bucketpolicy.SetIamPolicy(gcp.StringValue(cr.Spec.ForProvider.Bucket), &storage.Policy{}).
		Context(ctx).Do(); err != nil {
		return errors.Wrap(err, errSetPolicy)
	}
	return nil
}
