// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parametermanager

import (
	"context"
	"fmt"

	krm "github.com/GoogleCloudPlatform/k8s-config-connector/apis/parametermanager/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"

	// TODO(contributor): Update the import with the google cloud client
	gcp "cloud.google.com/go/parametermanager/apiv1"

	// TODO(contributor): Update the import with the google cloud client api protobuf
	parametermanagerpb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	"google.golang.org/api/option"
	"google.golang.org/genproto/protobuf/field_mask"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	registry.RegisterModel(krm.ParameterManagerParameterVersionGVK, NewParameterVersionModel)
}

func NewParameterVersionModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &modelParameterVersion{config: *config}, nil
}

var _ directbase.Model = &modelParameterVersion{}

type modelParameterVersion struct {
	config config.ControllerConfig
}

func (m *modelParameterVersion) client(ctx context.Context, location string) (*gcp.Client, error) {
	var opts []option.ClientOption
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}

	// Add regional endpoint if location is specified
	if location != "" && location != "global" {
		endpoint := fmt.Sprintf("parametermanager.%s.rep.googleapis.com:443", location)
		opts = append(opts, option.WithEndpoint(endpoint))
	}

	gcpClient, err := gcp.NewRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building ParameterVersion client: %w", err)
	}
	return gcpClient, err
}

func (m *modelParameterVersion) AdapterForObject(ctx context.Context, reader client.Reader, u *unstructured.Unstructured) (directbase.Adapter, error) {
	obj := &krm.ParameterManagerParameterVersion{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := krm.NewParameterVersionIdentity(ctx, reader, obj, u)
	if err != nil {
		return nil, err
	}

	location := id.Parent().Parent().Location
	if location == "" {
		location = "global"
	}

	// Get parametermanager GCP client
	gcpClient, err := m.client(ctx, location)
	if err != nil {
		return nil, err
	}
	return &ParameterVersionAdapter{
		id:        id,
		gcpClient: gcpClient,
		desired:   obj,
	}, nil
}

func (m *modelParameterVersion) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	// TODO: Support URLs
	return nil, nil
}

type ParameterVersionAdapter struct {
	id        *krm.ParameterVersionIdentity
	gcpClient *gcp.Client
	desired   *krm.ParameterManagerParameterVersion
	actual    *parametermanagerpb.ParameterVersion
}

var _ directbase.Adapter = &ParameterVersionAdapter{}

// Find retrieves the GCP resource.
// Return true means the object is found. This triggers Adapter `Update` call.
// Return false means the object is not found. This triggers Adapter `Create` call.
// Return a non-nil error requeues the requests.
func (a *ParameterVersionAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting ParameterVersion", "name", a.id)

	req := &parametermanagerpb.GetParameterVersionRequest{Name: a.id.String()}
	parameterversionpb, err := a.gcpClient.GetParameterVersion(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ParameterVersion %q: %w", a.id, err)
	}

	a.actual = parameterversionpb
	return true, nil
}

// Create creates the resource in GCP based on `spec` and update the Config Connector object `status` based on the GCP response.
func (a *ParameterVersionAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating ParameterVersion", "name", a.id)
	mapCtx := &direct.MapContext{}

	desired := a.desired.DeepCopy()
	resource := ParameterManagerParameterVersionSpec_ToProto(mapCtx, &desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	// TODO(contributor): Complete the gcp "CREATE" or "INSERT" request.
	req := &parametermanagerpb.CreateParameterVersionRequest{
		Parent:             a.id.Parent().String(),
		ParameterVersion:   resource,
		ParameterVersionId: a.id.ID(),
	}
	created, err := a.gcpClient.CreateParameterVersion(ctx, req)
	if err != nil {
		return fmt.Errorf("creating ParameterVersion %s: %w", a.id, err)
	}
	log.V(2).Info("successfully created ParameterVersion", "name", a.id)

	status := &krm.ParameterManagerParameterVersionStatus{}
	status.ObservedState = ParameterManagerParameterVersionObservedState_FromProto(mapCtx, created)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return createOp.UpdateStatus(ctx, status, nil)
}

// Update updates the resource in GCP based on `spec` and update the Config Connector object `status` based on the GCP response.
func (a *ParameterVersionAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating SecretVersion", "name", a.id)

	var updated *parametermanagerpb.ParameterVersion
	var err error

	disableParameterVersion := *a.desired.Spec.Disabled
	if a.actual.Disabled != disableParameterVersion {
		if disableParameterVersion {
			log.V(2).Info("disabling the parameter version", "name", a.id)
		} else {
			log.V(2).Info("enabling the parameter version", "name", a.id)
		}
		req := &parametermanagerpb.UpdateParameterVersionRequest{
			UpdateMask: &field_mask.FieldMask{
				Paths: []string{"disabled"},
			},
			ParameterVersion: &parametermanagerpb.ParameterVersion{
				Name:     a.id.String(),
				Disabled: disableParameterVersion,
			},
		}
		updated, err = a.gcpClient.UpdateParameterVersion(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to update parameter version %s: %w", a.id, err)
		}
	} else {
		log.V(2).Info("disabled is alredy %s for parameter version", "name", a.id)
	}

	mapCtx := &direct.MapContext{}
	status := &krm.ParameterManagerParameterVersionStatus{}
	status.ObservedState = ParameterManagerParameterVersionObservedState_FromProto(mapCtx, updated)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	return updateOp.UpdateStatus(ctx, status, nil)
}

// Export maps the GCP object to a Config Connector resource `spec`.
func (a *ParameterVersionAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.ParameterManagerParameterVersion{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(ParameterManagerParameterVersionSpec_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	obj.Spec.ParameterRef = &krm.ParameterRef{External: a.id.Parent().String()}

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.SetName(a.id.ID())
	u.SetGroupVersionKind(krm.ParameterManagerParameterVersionGVK)

	u.Object = uObj
	return u, nil
}

// Delete the resource from GCP service when the corresponding Config Connector resource is deleted.
func (a *ParameterVersionAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting ParameterVersion", "name", a.id)

	req := &parametermanagerpb.DeleteParameterVersionRequest{Name: a.id.String()}
	err := a.gcpClient.DeleteParameterVersion(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			// Return success if not found (assume it was already deleted).
			log.V(2).Info("skipping delete for non-existent ParameterVersion, assuming it was already deleted", "name", a.id)
			return true, nil
		}
		return false, fmt.Errorf("deleting ParameterVersion %s: %w", a.id, err)
	}
	log.V(2).Info("successfully deleted ParameterVersion", "name", a.id)
	return true, nil
}
