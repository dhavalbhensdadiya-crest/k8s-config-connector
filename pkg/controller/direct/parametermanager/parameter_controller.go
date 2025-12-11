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
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/label"

	// TODO(contributor): Update the import with the google cloud client
	gcp "cloud.google.com/go/parametermanager/apiv1"

	// TODO(contributor): Update the import with the google cloud client api protobuf
	parametermanagerpb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	registry.RegisterModel(krm.ParameterManagerParameterGVK, NewParameterModel)
}

func NewParameterModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &modelParameter{config: *config}, nil
}

var _ directbase.Model = &modelParameter{}

type modelParameter struct {
	config config.ControllerConfig
}

func (m *modelParameter) client(ctx context.Context, location string) (*gcp.Client, error) {
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
		return nil, fmt.Errorf("building Parameter client: %w", err)
	}
	return gcpClient, err
}

func (m *modelParameter) AdapterForObject(ctx context.Context, reader client.Reader, u *unstructured.Unstructured) (directbase.Adapter, error) {
	obj := &krm.ParameterManagerParameter{}
	copied := u.DeepCopy()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(copied.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := krm.NewParameterIdentity(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	if err = normalizeExternal(ctx, reader, copied, obj); err != nil {
		return nil, err
	}

	obj.Labels = label.NewGCPLabelsFromK8sLabels(u.GetLabels())

	location := obj.Spec.ProjectAndLocationRef.Location
	if location == "" {
		location = "global"
	}

	// Get parametermanager GCP client
	gcpClient, err := m.client(ctx, location)
	if err != nil {
		return nil, err
	}
	return &ParameterAdapter{
		id:        id,
		gcpClient: gcpClient,
		desired:   obj,
	}, nil
}

func (m *modelParameter) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	// TODO: Support URLs
	return nil, nil
}

type ParameterAdapter struct {
	id        *krm.ParameterIdentity
	gcpClient *gcp.Client
	desired   *krm.ParameterManagerParameter
	actual    *parametermanagerpb.Parameter
}

var _ directbase.Adapter = &ParameterAdapter{}

func normalizeExternal(ctx context.Context, reader client.Reader, src client.Object, parameter *krm.ParameterManagerParameter) error {
	if parameter.Spec.KMSKeyRef != nil {
		kmsKeyRef := parameter.Spec.KMSKeyRef
		kmsKeyRef, err := refs.ResolveKMSCryptoKeyRef(ctx, reader, src, kmsKeyRef)
		if err != nil {
			return err
		}
		parameter.Spec.KMSKeyRef = kmsKeyRef
	}
	return nil
}

// Find retrieves the GCP resource.
// Return true means the object is found. This triggers Adapter `Update` call.
// Return false means the object is not found. This triggers Adapter `Create` call.
// Return a non-nil error requeues the requests.
func (a *ParameterAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting Parameter", "name", a.id)

	req := &parametermanagerpb.GetParameterRequest{Name: a.id.String()}
	parameterpb, err := a.gcpClient.GetParameter(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting Parameter %q: %w", a.id, err)
	}

	a.actual = parameterpb
	return true, nil
}

// Create creates the resource in GCP based on `spec` and update the Config Connector object `status` based on the GCP response.
func (a *ParameterAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating Parameter", "name", a.id)
	mapCtx := &direct.MapContext{}

	desired := a.desired.DeepCopy()
	resource := ParameterManagerParameterSpec_ToProto(mapCtx, &desired.Spec)
	resource.Labels = label.NewGCPLabelsFromK8sLabels(desired.Labels)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	// TODO(contributor): Complete the gcp "CREATE" or "INSERT" request.
	req := &parametermanagerpb.CreateParameterRequest{
		Parent:      a.id.Parent().String(),
		ParameterId: a.id.ID(),
		Parameter:   resource,
	}
	created, err := a.gcpClient.CreateParameter(ctx, req)
	if err != nil {
		return fmt.Errorf("creating Parameter %s: %w", a.id, err)
	}

	log.V(2).Info("successfully created Parameter", "name", a.id)

	status := &krm.ParameterManagerParameterStatus{}
	status.ObservedState = ParameterManagerParameterObservedState_FromProto(mapCtx, created)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return createOp.UpdateStatus(ctx, status, nil)
}

// Update updates the resource in GCP based on `spec` and update the Config Connector object `status` based on the GCP response.
func (a *ParameterAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating Parameter", "name", a.id)
	mapCtx := &direct.MapContext{}

	desiredPb := ParameterManagerParameterSpec_ToProto(mapCtx, &a.desired.DeepCopy().Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	desiredPb.Name = a.id.String()
	desiredPb.Labels = label.NewGCPLabelsFromK8sLabels(a.desired.Labels)
	paths := make(sets.Set[string])
	// Option 1: This option is good for proto that has `field_mask` for output-only, immutable, required/optional.
	// TODO(contributor): If choosing this option, remove the "Option 2" code.

	var err error
	paths, err = common.CompareProtoMessage(desiredPb, a.actual, common.BasicDiff)
	if err != nil {
		return err
	}

	// // Option 2: manually add all mutable fields.
	// // TODO(contributor): If choosing this option, remove the "Option 1" code.
	// {
	// 	if !reflect.DeepEqual(a.desired.Spec.DisplayName, a.actual.DisplayName) {
	// 		paths = paths.Insert("display_name")
	// 	}
	// }

	if len(paths) == 0 {
		log.V(2).Info("no field needs update", "name", a.id)
		return nil
	}
	log.V(2).Info("fields need update", "name", a.id, "paths", paths)
	updateMask := &fieldmaskpb.FieldMask{
		Paths: sets.List(paths),
	}

	// TODO(contributor): Complete the gcp "UPDATE" or "PATCH" request.
	req := &parametermanagerpb.UpdateParameterRequest{
		// Name:       a.id.String(),
		UpdateMask: updateMask,
		Parameter:  desiredPb,
	}

	updated, err := a.gcpClient.UpdateParameter(ctx, req)
	if err != nil {
		return fmt.Errorf("updating Parameter %s: %w", a.id, err)
	}
	log.V(2).Info("successfully updated Parameter", "name", a.id)

	status := &krm.ParameterManagerParameterStatus{}
	status.ObservedState = ParameterManagerParameterObservedState_FromProto(mapCtx, updated)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	return updateOp.UpdateStatus(ctx, status, nil)
}

// Export maps the GCP object to a Config Connector resource `spec`.
func (a *ParameterAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.ParameterManagerParameter{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(ParameterManagerParameterSpec_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}
	obj.Spec.ProjectRef = &refs.ProjectRef{External: a.id.Parent().ProjectID}
	obj.Spec.Location = a.id.Parent().Location
	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.SetName(a.id.ID())
	u.SetGroupVersionKind(krm.ParameterManagerParameterGVK)

	u.Object = uObj
	return u, nil
}

// Delete the resource from GCP service when the corresponding Config Connector resource is deleted.
func (a *ParameterAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting Parameter", "name", a.id)

	req := &parametermanagerpb.DeleteParameterRequest{Name: a.id.String()}
	err := a.gcpClient.DeleteParameter(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			// Return success if not found (assume it was already deleted).
			log.V(2).Info("skipping delete for non-existent Parameter, assuming it was already deleted", "name", a.id)
			return true, nil
		}
		return false, fmt.Errorf("deleting Parameter %s: %w", a.id, err)
	}
	log.V(2).Info("successfully deleted Parameter", "name", a.id)
	return true, nil
}
