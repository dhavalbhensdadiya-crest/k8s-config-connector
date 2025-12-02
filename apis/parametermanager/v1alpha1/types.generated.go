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

// +generated:types
// krm.group: parametermanager.cnrm.cloud.google.com
// krm.version: v1alpha1
// proto.service: google.cloud.parametermanager.v1
// resource: ParameterManagerParameter:Parameter

package v1alpha1

// +kcc:proto=google.cloud.parametermanager.v1.Parameter
type Parameter struct {
	// Identifier. [Output only] The resource name of the Parameter in the format
	//  `projects/*/locations/*/parameters/*`.
	// +kcc:proto:field=google.cloud.parametermanager.v1.Parameter.name
	Name *string `json:"name,omitempty"`

	// Optional. Labels as key value pairs
	// +kcc:proto:field=google.cloud.parametermanager.v1.Parameter.labels
	Labels map[string]string `json:"labels,omitempty"`

	// Optional. Specifies the format of a Parameter.
	// +kcc:proto:field=google.cloud.parametermanager.v1.Parameter.format
	Format *string `json:"format,omitempty"`

	// Optional. Customer managed encryption key (CMEK) to use for encrypting the
	//  Parameter Versions. If not set, the default Google-managed encryption key
	//  will be used. Cloud KMS CryptoKeys must reside in the same location as the
	//  Parameter. The expected format is
	//  `projects/*/locations/*/keyRings/*/cryptoKeys/*`.
	// +kcc:proto:field=google.cloud.parametermanager.v1.Parameter.kms_key
	KMSKey *string `json:"kmsKey,omitempty"`
}

// +kcc:proto=google.iam.v1.ResourcePolicyMember
type ResourcePolicyMember struct {
}

// +kcc:observedstate:proto=google.cloud.parametermanager.v1.Parameter
type ParameterObservedState struct {
	// Output only. [Output only] Create time stamp
	// +kcc:proto:field=google.cloud.parametermanager.v1.Parameter.create_time
	CreateTime *string `json:"createTime,omitempty"`

	// Output only. [Output only] Update time stamp
	// +kcc:proto:field=google.cloud.parametermanager.v1.Parameter.update_time
	UpdateTime *string `json:"updateTime,omitempty"`

	// Output only. [Output-only] policy member strings of a Google Cloud
	//  resource.
	// +kcc:proto:field=google.cloud.parametermanager.v1.Parameter.policy_member
	PolicyMember *ResourcePolicyMember `json:"policyMember,omitempty"`
}

// +kcc:observedstate:proto=google.iam.v1.ResourcePolicyMember
type ResourcePolicyMemberObservedState struct {
	// IAM policy binding member referring to a Google Cloud resource by
	//  user-assigned name (https://google.aip.dev/122). If a resource is deleted
	//  and recreated with the same name, the binding will be applicable to the new
	//  resource.
	//
	//  Example:
	//  `principal://parametermanager.googleapis.com/projects/12345/name/locations/us-central1-a/parameters/my-parameter`
	// +kcc:proto:field=google.iam.v1.ResourcePolicyMember.iam_policy_name_principal
	IAMPolicyNamePrincipal *string `json:"iamPolicyNamePrincipal,omitempty"`

	// IAM policy binding member referring to a Google Cloud resource by
	//  system-assigned unique identifier (https://google.aip.dev/148#uid). If a
	//  resource is deleted and recreated with the same name, the binding will not
	//  be applicable to the new resource
	//
	//  Example:
	//  `principal://parametermanager.googleapis.com/projects/12345/uid/locations/us-central1-a/parameters/a918fed5`
	// +kcc:proto:field=google.iam.v1.ResourcePolicyMember.iam_policy_uid_principal
	IAMPolicyUidPrincipal *string `json:"iamPolicyUidPrincipal,omitempty"`
}
