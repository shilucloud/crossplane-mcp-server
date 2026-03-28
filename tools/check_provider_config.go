package tools

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type ProviderConfig struct {
	Name                string
	Users               int64
	CredentialType      string
	SecretName          string
	CredentialNamespace string
	SecretExists        bool
	Ready               string
	Synced              string
	Conditions          []Condition
}

func CheckProviderConfig(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	clientset kubernetes.Interface,
	restMapper meta.RESTMapper,
	kind string,
	group string,
	name string,
	namespace string,
) (*ProviderConfig, error) {
	result := &ProviderConfig{}

	gk := schema.GroupKind{
		Group: group,
		Kind:  kind,
	}

	mapping, err := restMapper.RESTMapping(gk)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    mapping.Resource.Group,
		Version:  mapping.Resource.Version,
		Resource: mapping.Resource.Resource,
	}

	// fetch the ProviderConfig — try namespaced first then cluster-scoped
	var pcObj map[string]interface{}
	if namespace != "" {
		o, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		pcObj = o.Object
	} else {
		o, err := dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		pcObj = o.Object
	}

	result.Name = getNestedString(pcObj, "metadata", "name")
	result.CredentialType = getNestedString(pcObj, "spec", "credentials", "source")
	result.SecretName = getNestedString(pcObj, "spec", "credentials", "secretRef", "name")
	result.CredentialNamespace = getNestedString(pcObj, "spec", "credentials", "secretRef", "namespace")
	result.Users = getNestedInt64(pcObj, "status", "users")
	result.Ready = resolveConditionStatus(pcObj, "Ready")
	result.Synced = resolveConditionStatus(pcObj, "Synced")

	// check if credentials secret actually exists
	result.SecretExists = false
	if result.SecretName != "unknown" && result.SecretName != "" {
		ns := result.CredentialNamespace
		if ns == "unknown" || ns == "" {
			ns = "default"
		}
		_, err := clientset.CoreV1().Secrets(ns).Get(ctx, result.SecretName, metav1.GetOptions{})
		result.SecretExists = err == nil
	}

	// extract conditions
	if status, ok := pcObj["status"].(map[string]interface{}); ok {
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conditions {
				cond, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				result.Conditions = append(result.Conditions, Condition{
					Type:               getString(cond, "type"),
					Status:             getString(cond, "status"),
					Reason:             getString(cond, "reason"),
					Message:            getString(cond, "message"),
					LastTransitionTime: getString(cond, "lastTransitionTime"),
				})
			}
		}
	}

	return result, nil
}
