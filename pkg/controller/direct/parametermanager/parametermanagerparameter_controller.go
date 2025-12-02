// Copyright 2024 Google LLC
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

// +tool:controller
// proto.service: google.cloud.parametermanager.v1.ParameterManager
// proto.message: google.cloud.parametermanager.v1.Parameter
// crd.type: ParameterManagerParameter
// crd.version: v1alpha1

package parametermanager

import (
	"context"
	"fmt"

	parametermanager "cloud.google.com/go/parametermanager/apiv1"
	parametermanagerpb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/parent"
	krm "github.com/GoogleCloudPlatform/k8s-config-connector/apis/parametermanager/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
)

func init() {
	registry.RegisterModel(krm.ParameterManagerParameterGVK, NewParameterModel)
}

func NewParameterModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &ParameterModel{config: *config}, nil
}

var _ directbase.Model = &ParameterModel{}

type ParameterModel struct {
	config config.ControllerConfig
}

func (m *ParameterModel) client(ctx context.Context, projectID string) (*parametermanager.Client, error) {
	var opts []option.ClientOption

	config := m.config

	// the service requires that a quota project be set
	if !config.UserProjectOverride || config.BillingProject == "" {
		config.UserProjectOverride = true
		config.BillingProject = projectID
	}

	opts, err := config.RESTClientOptions()
	if err != nil {
		return nil, err
	}

	gcpClient, err := parametermanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building parametermanager client: %w", err)
	}

	return gcpClient, err
}

func (m *ParameterModel) AdapterForObject(ctx context.Context, reader client.Reader, u *unstructured.Unstructured) (directbase.Adapter, error) {
	obj := &krm.ParameterManagerParameter{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := krm.NewParameterManagerParameterIdentity(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	gcpClient, err := m.client(ctx, id.Parent().ProjectID)
	if err != nil {
		return nil, err
	}

	return &parameterAdapter{
		gcpClient: gcpClient,
		id:        id,
		desired:   obj,
	}, nil
}

func (m *ParameterModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	// TODO: Support URLs
	return nil, nil
}

type parameterAdapter struct {
	gcpClient *parametermanager.Client
	id        *krm.ParameterManagerParameterIdentity
	desired   *krm.ParameterManagerParameter
	actual    *parametermanagerpb.Parameter
}

var _ directbase.Adapter = &parameterAdapter{}

func (a *parameterAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.Info("getting parametermanager parameter", "name", a.id)

	req := &parametermanagerpb.GetParameterRequest{Name: a.id.String()}
	actual, err := a.gcpClient.GetParameter(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting parametermanager parameter %q from gcp: %w", a.id.String(), err)
	}

	a.actual = actual
	return true, nil
}

func (a *parameterAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.Info("creating parametermanager parameter", "name", a.id)

	desired := a.desired.DeepCopy()
	desired.Name = a.id.String()

	req := &parametermanagerpb.CreateParameterRequest{
		Parent:      a.id.Parent().String(),
		Parameter:   &parametermanagerpb.Parameter{Name: desired.Name},
		ParameterId: a.id.ID(),
	}

	if desired.Spec.ResourceID != nil {
		req.ParameterId = *desired.Spec.ResourceID
	}

	_, err := a.gcpClient.CreateParameter(ctx, req)
	if err != nil {
		return fmt.Errorf("creating parametermanager parameter %s: %w", a.id.String(), err)
	}
	log.Info("successfully created parametermanager parameter in gcp", "name", a.id)

	status := &krm.ParameterManagerParameterStatus{}
	status.ExternalRef = direct.LazyPtr(a.id.String())

	mapCtx := &direct.MapContext{}
	status.ObservedState = ParameterManagerParameterObservedState_FromProto(mapCtx, a.actual)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	return createOp.UpdateStatus(ctx, status, nil)
}

func (a *parameterAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating parametermanager parameter", "name", a.id)

	updateMask := &fieldmaskpb.FieldMask{}
	updated := proto.Clone(a.actual).(*parametermanagerpb.Parameter)

	// TODO: Implement update logic based on desired.Spec changes
	// Example:
	// if !reflect.DeepEqual(a.actual.Labels, a.desired.Spec.Labels) {
	// 	updated.Labels = a.desired.Spec.Labels
	// 	updateMask.Paths = append(updateMask.Paths, "labels")
	// }

	if len(updateMask.Paths) == 0 {
		// no-op, just update obj status
		status := &krm.ParameterManagerParameterStatus{}
		status.ExternalRef = direct.LazyPtr(a.actual.Name)
		return updateOp.UpdateStatus(ctx, status, nil)
	}

	req := &parametermanagerpb.UpdateParameterRequest{
		Parameter:  updated,
		UpdateMask: updateMask,
	}

	updatedParameter, err := a.gcpClient.UpdateParameter(ctx, req)
	if err != nil {
		return fmt.Errorf("updating parametermanager parameter %s: %w", a.id, err)
	}
	log.V(2).Info("successfully updated parametermanager parameter", "name", a.id)

	status := &krm.ParameterManagerParameterStatus{}
	status.ExternalRef = direct.LazyPtr(updatedParameter.Name)
	mapCtx := &direct.MapContext{}
	status.ObservedState = ParameterManagerParameterObservedState_FromProto(mapCtx, a.actual)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	return updateOp.UpdateStatus(ctx, status, nil)
}

func (a *parameterAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.Info("deleting parametermanager parameter", "name", a.id)

	req := &parametermanagerpb.DeleteParameterRequest{Name: a.id.String()}
	_, err := a.gcpClient.DeleteParameter(ctx, req)
	if err != nil {
		return false, fmt.Errorf("deleting parametermanager parameter %s: %w", a.id.String(), err)
	}
	log.Info("successfully deleted parametermanager parameter", "name", a.id)

	return true, nil
}

// Export maps the GCP object to a Config Connector resource `spec`.
func (a *parameterAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.ParameterManagerParameter{}
	mapCtx := &direct.MapContext{}
	obj.Status.ObservedState = ParameterManagerParameterObservedState_FromProto(mapCtx, a.actual)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}
	obj.Spec.ProjectAndLocationRef = &parent.ProjectAndLocationRef{Project: a.id.Parent().ProjectID, Location: a.id.Parent().Location}
	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.SetName(a.id.ID())
	u.SetGroupVersionKind(krm.ParameterManagerParameterGVK)

	u.Object = uObj
	return u, nil
}

func ParameterManagerParameterObservedState_FromProto(ctx *direct.MapContext, in *parametermanagerpb.Parameter) *krm.ParameterManagerParameterObservedState {
	if in == nil {
		return nil
	}
	out := &krm.ParameterManagerParameterObservedState{}
	return out
}
