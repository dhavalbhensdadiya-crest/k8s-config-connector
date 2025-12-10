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

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ParameterVersionIdentity defines the resource reference to ParameterManagerParameterVersion, which "External" field
// holds the GCP identifier for the KRM object.
type ParameterVersionIdentity struct {
	parent *ParameterIdentity
	id     string
}

func (i *ParameterVersionIdentity) String() string {
	return i.parent.String() + "/versions/" + i.id
}

func (i *ParameterVersionIdentity) ID() string {
	return i.id
}

func (i *ParameterVersionIdentity) Parent() *ParameterIdentity {
	return i.parent
}

// New builds a ParameterVersionIdentity from the Config Connector ParameterVersion object.
func NewParameterVersionIdentity(ctx context.Context, reader client.Reader, obj *ParameterManagerParameterVersion, u *unstructured.Unstructured) (*ParameterVersionIdentity, error) {

	projectID, err := refsv1beta1.ResolveProjectID(ctx, reader, u)
	if err != nil {
		return nil, err
	}
	if projectID == "" {
		return nil, fmt.Errorf("cannot resolve project")
	}

	parameterExternal, err := obj.Spec.ParameterRef.NormalizedExternal(ctx, reader, obj.GetNamespace())
	if err != nil {
		return nil, err
	}
	parameterParent, parameterResourceId, err := ParseParameterExternal(parameterExternal)
	if err != nil {
		return nil, err
	}

	// If `spec.resourceID` is not empty, it means user wants to acquire the object.
	desiredVersionID := common.ValueOf(obj.Spec.ResourceID)

	externalRef := common.ValueOf(obj.Status.ExternalRef)
	if externalRef != "" {
		actualParameterIdentity, actualResourceId, err := ParseParameterVersionExternal(externalRef)
		if err != nil {
			return nil, err
		}
		if actualParameterIdentity.parent.ProjectID != parameterParent.ProjectID {
			return nil, fmt.Errorf("projectID in spec.parameterRef changed, expect %s, got %s", actualParameterIdentity.parent.ProjectID, parameterParent.ProjectID)
		}
		if actualParameterIdentity.parent.Location != parameterParent.Location {
			return nil, fmt.Errorf("location in spec.projectRef changed, expect %s, got %s", actualParameterIdentity.parent.Location, parameterParent.Location)
		}
		if actualParameterIdentity.id != parameterResourceId {
			return nil, fmt.Errorf("resourceID in spec.parameterRef changed, expect %s, got %s", actualResourceId, parameterResourceId)
		}
		if desiredVersionID != "" && actualResourceId != desiredVersionID {
			return nil, fmt.Errorf("cannot reset `metadata.name` or `spec.resourceID` to %s, since it has already assigned to %s",
				desiredVersionID, actualResourceId)
		}
		desiredVersionID = actualResourceId
	}

	return &ParameterVersionIdentity{
		parent: &ParameterIdentity{
			parent: parameterParent,
			id:     parameterResourceId,
		},
		id: desiredVersionID,
	}, nil
}

func ParseParameterVersionExternal(external string) (parent *ParameterIdentity, resourceID string, err error) {
	tokens := strings.Split(external, "/")
	if len(tokens) != 8 || tokens[0] != "projects" || tokens[2] != "locations" || tokens[4] != "parameters" || tokens[6] != "versions" {
		return nil, "", fmt.Errorf("format of ParameterManagerParameterVersion external=%q was not known (use projects/{{projectID}}/locations/{{location}}/parameters/{{parameterID}}/versions/{{versionID}})", external)
	}

	parameterParent := &ParameterParent{
		ProjectID: tokens[1],
		Location:  tokens[3],
	}

	parent = &ParameterIdentity{
		parent: parameterParent,
		id:     tokens[5],
	}

	resourceID = tokens[7]
	return parent, resourceID, nil
}
