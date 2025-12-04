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

package mockparametermanager

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/k8s-config-connector/mockgcp/common/projects"
	pb "github.com/GoogleCloudPlatform/k8s-config-connector/mockgcp/generated/mockgcp/cloud/parametermanager/v1"
)

type ParameterManagerV1 struct {
	*MockService

	pb.UnimplementedParameterManagerServer
}

// Creates a new [Parameter][google.cloud.parametermanager.v1.Parameter].
func (s *ParameterManagerV1) CreateParameter(ctx context.Context, req *pb.CreateParameterRequest) (*pb.Parameter, error) {
	parameterID := req.ParameterId
	if parameterID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ParameterId is required")
	}

	parent, err := projects.ParseProjectName(req.Parent)
	if err != nil {
		return nil, err
	}

	project, err := s.Projects.GetProject(parent)
	if err != nil {
		return nil, err
	}

	name := parameterName{
		Project:   project,
		Location:  "global", // Assuming global for now
		ParameterName: parameterID,
	}
	fqn := name.String()

	obj := proto.Clone(req.Parameter).(*pb.Parameter)
	obj.Name = fqn
	obj.CreateTime = timestamppb.Now()

	if err := s.storage.Create(ctx, fqn, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

// Gets metadata for a given [Parameter][google.cloud.parametermanager.v1.Parameter].
func (s *ParameterManagerV1) GetParameter(ctx context.Context, req *pb.GetParameterRequest) (*pb.Parameter, error) {
	name, err := s.parseParameterName(req.Name)
	if err != nil {
		return nil, err
	}

	var parameter pb.Parameter
	fqn := name.String()
	if err := s.storage.Get(ctx, fqn, &parameter); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Errorf(codes.NotFound, "Parameter [%s] not found.", fqn)
		}
		return nil, err
	}

	return &parameter, nil
}

// Deletes a [Parameter][google.cloud.parametermanager.v1.Parameter].
func (s *ParameterManagerV1) DeleteParameter(ctx context.Context, req *pb.DeleteParameterRequest) (*emptypb.Empty, error) {
	name, err := s.parseParameterName(req.Name)
	if err != nil {
		return nil, err
	}

	fqn := name.String()

	oldObj := &pb.Parameter{}
	if err := s.storage.Delete(ctx, fqn, oldObj); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// Update metadata for a given [Parameter][google.cloud.parametermanager.v1.Parameter].
func (s *ParameterManagerV1) UpdateParameter(ctx context.Context, req *pb.UpdateParameterRequest) (*pb.Parameter, error) {
	name, err := s.parseParameterName(req.Parameter.Name)
	if err != nil {
		return nil, err
	}
	fqn := name.String()
	existing := &pb.Parameter{}
	if err := s.storage.Get(ctx, fqn, existing); err != nil {
		return nil, err
	}

	updated := proto.Clone(existing).(*pb.Parameter)
	updated.Name = name.String()

	// Required. The update mask applies to the resource.
	paths := req.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "update_mask is required")
	}
	for _, path := range paths {
		switch path {
		case "labels":
			updated.Labels = req.Parameter.GetLabels()
		default:
			return nil, status.Errorf(codes.InvalidArgument, "update_mask path %q not valid", path)
		}
	}

	if err := s.storage.Update(ctx, fqn, updated); err != nil {
		return nil, err
	}
	return updated, nil
}

// Lists [Parameter][google.cloud.parametermanager.v1.Parameter].
func (s *ParameterManagerV1) ListParameters(ctx context.Context, req *pb.ListParametersRequest) (*pb.ListParametersResponse, error) {
	project, err := projects.ParseProjectName(req.Parent)
	if err != nil {
		return nil, err
	}

	p, err := s.Projects.GetProject(project)
	if err != nil {
		return nil, err
	}

	parent := "projects/" + p.ID + "/locations/" + "global"

	var parameters []*pb.Parameter
	if err := s.storage.List(ctx, parent, func(obj *pb.Parameter) {
		parameters = append(parameters, obj)
	}); err != nil {
		return nil, err
	}

	return &pb.ListParametersResponse{
		Parameters: parameters,
	}, nil
}

// Creates a new [ParameterVersion][google.cloud.parametermanager.v1.ParameterVersion].
func (s *ParameterManagerV1) CreateParameterVersion(ctx context.Context, req *pb.CreateParameterVersionRequest) (*pb.ParameterVersion, error) {
	parameterVersionID := req.ParameterVersionId
	if parameterVersionID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ParameterVersionId is required")
	}

	parent, err := s.parseParameterName(req.Parent)
	if err != nil {
		return nil, err
	}

	name := parameterVersionName{
		parameterName: *parent,
		Version:       parameterVersionID,
	}
	fqn := name.String()

	obj := proto.Clone(req.ParameterVersion).(*pb.ParameterVersion)
	obj.Name = fqn
	obj.CreateTime = timestamppb.Now()

	if err := s.storage.Create(ctx, fqn, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

// Gets metadata for a given [ParameterVersion][google.cloud.parametermanager.v1.ParameterVersion].
func (s *ParameterManagerV1) GetParameterVersion(ctx context.Context, req *pb.GetParameterVersionRequest) (*pb.ParameterVersion, error) {
	name, err := s.parseParameterVersionName(req.Name)
	if err != nil {
		return nil, err
	}

	var parameterVersion pb.ParameterVersion
	fqn := name.String()
	if err := s.storage.Get(ctx, fqn, &parameterVersion); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Errorf(codes.NotFound, "ParameterVersion [%s] not found.", fqn)
		}
		return nil, err
	}

	return &parameterVersion, nil
}

// Deletes a [ParameterVersion][google.cloud.parametermanager.v1.ParameterVersion].
func (s *ParameterManagerV1) DeleteParameterVersion(ctx context.Context, req *pb.DeleteParameterVersionRequest) (*emptypb.Empty, error) {
	name, err := s.parseParameterVersionName(req.Name)
	if err != nil {
		return nil, err
	}

	fqn := name.String()

	oldObj := &pb.ParameterVersion{}
	if err := s.storage.Delete(ctx, fqn, oldObj); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// Update metadata for a given [ParameterVersion][google.cloud.parametermanager.v1.ParameterVersion].
func (s *ParameterManagerV1) UpdateParameterVersion(ctx context.Context, req *pb.UpdateParameterVersionRequest) (*pb.ParameterVersion, error) {
	name, err := s.parseParameterVersionName(req.ParameterVersion.Name)
	if err != nil {
		return nil, err
	}
	fqn := name.String()
	existing := &pb.ParameterVersion{}
	if err := s.storage.Get(ctx, fqn, existing); err != nil {
		return nil, err
	}

	updated := proto.Clone(existing).(*pb.ParameterVersion)
	updated.Name = name.String()

	// Required. The update mask applies to the resource.
	paths := req.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "update_mask is required")
	}
	for _, path := range paths {
		switch path {
			if err := s.storage.Update(ctx, fqn, updated); err != nil {
		return nil, err
	}
	return updated, nil
}

// Lists [ParameterVersion][google.cloud.parametermanager.v1.ParameterVersion].
func (s *ParameterManagerV1) ListParameterVersions(ctx context.Context, req *pb.ListParameterVersionsRequest) (*pb.ListParameterVersionsResponse, error) {
	parent, err := s.parseParameterName(req.Parent)
	if err != nil {
		return nil, err
	}

	var parameterVersions []*pb.ParameterVersion
	if err := s.storage.List(ctx, parent.String(), func(obj *pb.ParameterVersion) {
		parameterVersions = append(parameterVersions, obj)
	}); err != nil {
		return nil, err
	}

	return &pb.ListParameterVersionsResponse{
		ParameterVersions: parameterVersions,
	}, nil
}


func (n *parameterVersionName) String() string {
	return n.parameterName.String() + "/versions/" + n.Version
}

func (s *MockService) parseParameterVersionName(name string) (*parameterVersionName, error) {
	tokens := strings.Split(name, "/")
	if len(tokens) == 8 && tokens[0] == "projects" && tokens[2] == "locations" && tokens[4] == "parameters" && tokens[6] == "versions" {
		parent, err := s.parseParameterName(strings.Join(tokens[0:6], "/"))
		if err != nil {
			return nil, err
		}
		name := &parameterVersionName{
			parameterName: *parent,
			Version:       tokens[7],
		}

		return name, nil
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "name %q is not valid", name)
	}
}

type parameterName struct {
	Project    *projects.ProjectData
	Location   string
	ParameterName string
}

func (n *parameterName) String() string {
	return "projects/" + n.Project.ID + "/locations/" + n.Location + "/parameters/" + n.ParameterName
}

// parseParameterName parses a string into a parameterName.
// The expected form is projects/<projectID>/locations/<location>/parameters/<parameterName>
func (s *MockService) parseParameterName(name string) (*parameterName, error) {
	tokens := strings.Split(name, "/")
	if len(tokens) == 6 && tokens[0] == "projects" && tokens[2] == "locations" && tokens[4] == "parameters" {
		project, err := s.Projects.GetProject(&projects.ProjectData{ID: tokens[1]})
		if err != nil {
			return nil, err
		}

		name := &parameterName{
			Project:    project,
			Location:   tokens[3],
			ParameterName: tokens[5],
		}

		return name, nil
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "name %q is not valid", name)
	}
}
