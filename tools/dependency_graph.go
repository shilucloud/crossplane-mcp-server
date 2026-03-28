package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type NodeStatus string

const (
	NodeHealthy   NodeStatus = "✓"
	NodeUnhealthy NodeStatus = "❌"
	NodeUnknown   NodeStatus = "?"
	NodeMissing   NodeStatus = "✗"
)

type DependencyNode struct {
	Name     string
	Kind     string
	Status   NodeStatus
	Message  string
	Children []*DependencyNode
}

func BuildDependencyGraph(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	clientset kubernetes.Interface,
	group, version, resource, name, namespace string,
) (*DependencyNode, error) {

	// root node — the XR
	root := &DependencyNode{
		Name: name,
		Kind: "XR",
	}

	// get XR tree — we already have all this data
	tree, err := GetXRTree(ctx, dynamicClient, group, version, resource, name, namespace)
	if err != nil {
		root.Status = NodeMissing
		root.Message = err.Error()
		return root, nil
	}

	// set XR status
	root.Status = resolveNodeStatus(tree.XRReady, tree.XRSynced)
	root.Message = fmt.Sprintf("Ready=%s Synced=%s", tree.XRReady, tree.XRSynced)

	// composition node
	compNode := &DependencyNode{
		Name: tree.CompositionInfo.Name,
		Kind: "Composition",
	}

	if tree.CompositionInfo.Name == "none selected" || tree.CompositionInfo.Name == "" {
		compNode.Status = NodeMissing
		compNode.Message = "no composition selected"
	} else {
		compNode.Status = NodeHealthy

		// function nodes under composition
		for _, step := range tree.CompositionInfo.Pipeline {
			stepMap, ok := step.(map[string]interface{})
			if !ok {
				continue
			}
			funcName := getNestedString(stepMap, "functionRef", "name")
			if funcName == "unknown" || funcName == "" {
				continue
			}

			funcNode := &DependencyNode{
				Name: funcName,
				Kind: "Function",
			}

			// check if function is healthy
			fn, err := dynamicClient.Resource(functionGVR).Get(ctx, funcName, metaGetOptions())
			if err != nil {
				funcNode.Status = NodeMissing
				funcNode.Message = "function not installed"
			} else {
				healthy := resolveConditionStatus(fn.Object, "Healthy")
				if healthy == "True" {
					funcNode.Status = NodeHealthy
				} else {
					funcNode.Status = NodeUnhealthy
					funcNode.Message = healthy
				}
			}

			compNode.Children = append(compNode.Children, funcNode)
		}
	}

	root.Children = append(root.Children, compNode)

	// MR nodes
	for _, mr := range tree.MRs {
		mrNode := &DependencyNode{
			Name: fmt.Sprintf("%s/%s", mr.Kind, mr.Name),
			Kind: "ManagedResource",
		}

		if mr.Ready == "NotFound" || mr.Synced == "NotFound" {
			mrNode.Status = NodeMissing
			mrNode.Message = "resource not created yet"
		} else {
			mrNode.Status = resolveNodeStatus(mr.Ready, mr.Synced)
			mrNode.Message = fmt.Sprintf("Ready=%s Synced=%s", mr.Ready, mr.Synced)
		}

		// ProviderConfig node under MR
		// ProviderConfig node under MR
		if mr.ProviderConfigName != "" && mr.ProviderConfigName != "unknown" {
			pcNode := &DependencyNode{
				Name: mr.ProviderConfigName,
				Kind: "ProviderConfig",
			}

			// if no conditions, check if it exists — existence = healthy enough
			if mr.ProviderConfigInfo.Ready == "" || mr.ProviderConfigInfo.Ready == "Unknown" {
				// ProviderConfig exists (we have the name) but has no conditions
				// treat as healthy if secret exists
				pcNode.Status = NodeHealthy
				pcNode.Message = fmt.Sprintf("users: %d", 1) // users field
			} else if mr.ProviderConfigInfo.Ready == "True" {
				pcNode.Status = NodeHealthy
			} else {
				pcNode.Status = NodeUnhealthy
				pcNode.Message = mr.ProviderConfigInfo.Ready
			}

			// Secret node under ProviderConfig
			secretName := findProviderConfigSecret(
				ctx, dynamicClient, mr.Group, mr.ProviderConfigName, namespace)

			if secretName != "" {
				secretNode := &DependencyNode{
					Name: secretName,
					Kind: "Secret",
				}
				_, err := clientset.CoreV1().Secrets(namespace).Get(
					ctx, secretName, metaGetOptions())
				if err != nil {
					secretNode.Status = NodeMissing
					secretNode.Message = "secret not found — credentials missing"
				} else {
					secretNode.Status = NodeHealthy
				}
				pcNode.Children = append(pcNode.Children, secretNode)
			}

			mrNode.Children = append(mrNode.Children, pcNode)
		}

		root.Children = append(root.Children, mrNode)
	}

	return root, nil
}
func PrintDependencyGraph(node *DependencyNode, prefix string, isLast bool) string {
	var sb strings.Builder

	if prefix == "" {
		// root node
		sb.WriteString(fmt.Sprintf("%s %s [%s]", node.Status, node.Name, node.Kind))
	} else {
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		sb.WriteString(fmt.Sprintf("%s%s%s %s [%s]", prefix, connector, node.Status, node.Name, node.Kind))
	}

	if node.Message != "" {
		sb.WriteString(fmt.Sprintf(" — %s", node.Message))
	}
	sb.WriteString("\n")

	// calculate child prefix
	var childPrefix string
	if prefix == "" {
		childPrefix = ""
	} else if isLast {
		childPrefix = prefix + "    "
	} else {
		childPrefix = prefix + "│   "
	}

	for i, child := range node.Children {
		isLastChild := i == len(node.Children)-1
		sb.WriteString(PrintDependencyGraph(child, childPrefix+"    ", isLastChild))
	}

	return sb.String()
}

func resolveNodeStatus(ready, synced string) NodeStatus {
	if ready == "True" && synced == "True" {
		return NodeHealthy
	}
	if strings.HasPrefix(ready, "False") || strings.HasPrefix(synced, "False") {
		return NodeUnhealthy
	}
	return NodeUnknown
}

func findProviderConfigSecret(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	mrGroup, pcName, namespace string,
) string {
	pcGroup := mrGroupToProviderConfigGroup(mrGroup)
	pcGVR := schemaGVR(pcGroup, "v1beta1", "providerconfigs")

	var obj map[string]interface{}
	if namespace != "" {
		o, err := dynamicClient.Resource(pcGVR).Namespace(namespace).Get(
			ctx, pcName, metaGetOptions())
		if err != nil {
			return ""
		}
		obj = o.Object
	} else {
		o, err := dynamicClient.Resource(pcGVR).Get(ctx, pcName, metaGetOptions())
		if err != nil {
			return ""
		}
		obj = o.Object
	}

	return getNestedString(obj, "spec", "credentials", "secretRef", "name")
}

func metaGetOptions() metav1.GetOptions {
	return metav1.GetOptions{}
}

func schemaGVR(group, version, resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
}
