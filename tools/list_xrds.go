package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func ListXRDs(ctx context.Context, dynamicClient dynamic.Interface) (*XRDList, error) {
	result := &XRDList{}

	xrds, err := dynamicClient.Resource(XRDGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing XRDs: %w", err)
	}

	result.Total = len(xrds.Items)

	// list all compositions once — to find which serve which XRD
	comps, _ := dynamicClient.Resource(compositionGVR).List(ctx, metav1.ListOptions{})

	for _, xrd := range xrds.Items {
		kind := getNestedString(xrd.Object, "spec", "names", "kind")
		group := getNestedString(xrd.Object, "spec", "group")
		scope := getNestedString(xrd.Object, "spec", "scope")
		version := getServedVersion(xrd.Object)
		established := resolveConditionStatus(xrd.Object, "Established") == "True"

		xi := XRDInfo{
			Name:        xrd.GetName(),
			Kind:        kind,
			Group:       group,
			Version:     version,
			Scope:       scope,
			Established: established,
		}

		if scope == "Namespaced" {
			result.Namespaced++
		} else {
			result.Cluster++
		}

		// count XR instances
		if version != "" {
			plural := getNestedString(xrd.Object, "spec", "names", "plural")
			xrGVR := schemaGVR(group, version, plural)

			var count int
			if scope == "Namespaced" {
				list, err := dynamicClient.Resource(xrGVR).Namespace("").List(ctx, metav1.ListOptions{})
				if err == nil {
					count = len(list.Items)
				}
			} else {
				list, err := dynamicClient.Resource(xrGVR).List(ctx, metav1.ListOptions{})
				if err == nil {
					count = len(list.Items)
				}
			}
			xi.XRCount = count
		}

		// find compositions that serve this XRD
		if comps != nil {
			for _, comp := range comps.Items {
				compKind := getNestedString(comp.Object, "spec", "compositeTypeRef", "kind")
				compAPIVersion := getNestedString(comp.Object, "spec", "compositeTypeRef", "apiVersion")
				compGroup, _ := splitAPIVersion(compAPIVersion)
				if compKind == kind && compGroup == group {
					xi.Compositions = append(xi.Compositions, comp.GetName())
				}
			}
		}

		result.XRDs = append(result.XRDs, xi)
	}

	return result, nil
}

func (x *XRDList) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total XRDs: %d | Namespaced: %d | Cluster: %d\n\n",
		x.Total, x.Namespaced, x.Cluster))

	for _, xrd := range x.XRDs {
		established := "✅"
		if !xrd.Established {
			established = "❌"
		}
		sb.WriteString(fmt.Sprintf("%s %s (%s)\n", established, xrd.Kind, xrd.Name))
		sb.WriteString(fmt.Sprintf("   group: %s | version: %s | scope: %s\n",
			xrd.Group, xrd.Version, xrd.Scope))
		sb.WriteString(fmt.Sprintf("   instances: %d | compositions: %v\n",
			xrd.XRCount, xrd.Compositions))
	}

	return sb.String()
}
