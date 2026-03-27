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
		//for _, resource := range getNestedSlice(composition.Object, "spec", "resources"){
		//	name := getNestedString(resource, )
		//}
		result = append(result, CompositionInfo{
			Name:     composition.GetName(),
			Mode:     getNestedString(composition.Object, "spec", "mode"),
			Pipeline: getNestedSlice(composition.Object, "spec", "pipeline"),
		})
	}

	return result, nil

}

//func GetComposition(ctx context.Context, dynamicClient dynamic.Interface, group, version, resource, name, namespace string) (*CompositionInfo, error) {
//}
