// in pkg/controller/direct/parametermanager/parametermanager_fuzzer.go

// +tool:fuzz-gen
// proto.message: google.cloud.parametermanager.v1.Parameter
// api.group: parametermanager.cnrm.cloud.google.com

package parametermanager

import (
	pb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/fuzztesting"
)

func init() {
	fuzztesting.RegisterKRMFuzzer(myNewResourceFuzzer())
}

func myNewResourceFuzzer() fuzztesting.KRMFuzzer {
	f := fuzztesting.NewKRMTypedFuzzer(&pb.Parameter{},
		ParameterManagerParameterSpec_FromProto, ParameterManagerParameterSpec_ToProto,
		ParameterManagerParameterObservedState_FromProto, ParameterManagerParameterObservedState_ToProto,
	)

	// Fields that exist in KCC spec but not in the proto
	// f.SpecFields.Insert(".annotations")

	// Fields that exist in KCC status but not in the proto
	// f.StatusFields.Insert(".create_time")
	// f.StatusFields.Insert(".uid")

	// Fields that are not yet implemented or have known issues
	f.Unimplemented_NotYetTriaged(".labels")
	f.Unimplemented_NotYetTriaged(".policy_member")

	return f
}
