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
	fuzztesting.RegisterKRMFuzzer(parameterManagerParameterFuzzer())
}

func parameterManagerParameterFuzzer() fuzztesting.KRMFuzzer {
	f := fuzztesting.NewKRMTypedFuzzer(&pb.Parameter{},
		ParameterManagerParameterSpec_FromProto, ParameterManagerParameterSpec_ToProto,
		ParameterManagerParameterObservedState_FromProto, ParameterManagerParameterObservedState_ToProto,
	)

	f.Unimplemented_LabelsAnnotations(".labels")
	f.Unimplemented_NotYetTriaged(".policy_member") // Output Only. Not yet figured how to handle this

	f.SpecFields.Insert(".format")
	f.SpecFields.Insert(".kms_key")

	f.StatusFields.Insert(".name")        // Output Only
	f.StatusFields.Insert(".create_time") // Output Only
	f.StatusFields.Insert(".update_time") // Output Only

	return f
}
