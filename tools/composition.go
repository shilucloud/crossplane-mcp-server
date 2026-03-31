package tools

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func ListComposition(ctx context.Context, dynamicClient dynamic.Interface) ([]CompositionInfo, error) {
	result := []CompositionInfo{}
	compositions, err := dynamicClient.Resource(compositionGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, composition := range compositions.Items {
		var pipelineInfos []PipelineInfo
		pipelineSteps := getNestedSlice(composition.Object, "spec", "pipeline")
		for _, step := range pipelineSteps {
			stepMap, ok := step.(map[string]interface{})
			if !ok {
				continue
			}

			// step name
			stepName := getNestedString(stepMap, "step")

			// Function name
			functionName := getNestedString(stepMap, "functionRef", "name")

			// Resources
			var resourcesInfo []PipelineResourceInfo
			resources := getNestedSlice(stepMap, "input", "resources")

			for _, res := range resources {
				resMap, ok := res.(map[string]interface{})
				if !ok {
					continue
				}

				resourcesInfo = append(resourcesInfo, PipelineResourceInfo{
					Name: getNestedString(resMap, "name"),
					Kind: getNestedString(resMap, "base", "kind"),
				})
			}
			pipelineInfos = append(pipelineInfos, PipelineInfo{
				StepName:     stepName,
				FunctionName: functionName,
				Resources:    resourcesInfo,
			})
		}
		result = append(result, CompositionInfo{
			Name:         composition.GetName(),
			Mode:         getNestedString(composition.Object, "spec", "mode"),
			PipelineInfo: pipelineInfos,
		})
	}

	return result, nil

}
