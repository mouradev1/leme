package resources

import (
	"context"
	"fmt"
	"sort"
	"unicode/utf8"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"leme/internal/cluster"
)

// ObjectData is the key/value payload of a ConfigMap or Secret, as text.
// Secret values arrive base64-decoded; keys that aren't valid UTF-8 (certs in
// DER, keystores...) are listed in Binary and are read-only in the UI.
type ObjectData struct {
	Data            map[string]string `json:"data"`
	Binary          []string          `json:"binary"`
	ResourceVersion string            `json:"resourceVersion"`
}

func GetData(ctx context.Context, s *cluster.Session, resource, ns, name string) (ObjectData, error) {
	out := ObjectData{Data: map[string]string{}}
	switch resource {
	case "configmaps":
		cm, err := s.Kube.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return out, err
		}
		for k, v := range cm.Data {
			out.Data[k] = v
		}
		for k := range cm.BinaryData {
			out.Binary = append(out.Binary, k)
		}
		out.ResourceVersion = cm.ResourceVersion
	case "secrets":
		sec, err := s.Kube.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return out, err
		}
		for k, v := range sec.Data {
			if utf8.Valid(v) {
				out.Data[k] = string(v)
			} else {
				out.Binary = append(out.Binary, k)
			}
		}
		out.ResourceVersion = sec.ResourceVersion
	default:
		return out, fmt.Errorf("data editing not supported for %s", resource)
	}
	sort.Strings(out.Binary)
	return out, nil
}

// SetData replaces the text keys (adds, edits, removes) and leaves binary keys
// untouched. resourceVersion is an optimistic lock: if someone changed the
// object since it was loaded, the update fails with a Conflict.
func SetData(ctx context.Context, s *cluster.Session, resource, ns, name string, data map[string]string, resourceVersion string) (string, error) {
	switch resource {
	case "configmaps":
		cm, err := s.Kube.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		cm.ResourceVersion = resourceVersion
		cm.Data = data
		out, err := s.Kube.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{FieldManager: FieldManager})
		if err != nil {
			return "", err
		}
		return out.ResourceVersion, nil
	case "secrets":
		sec, err := s.Kube.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		sec.ResourceVersion = resourceVersion
		next := map[string][]byte{}
		for k, v := range sec.Data {
			if !utf8.Valid(v) {
				next[k] = v // binary keys are preserved as-is
			}
		}
		for k, v := range data {
			next[k] = []byte(v)
		}
		sec.Data = next
		out, err := s.Kube.CoreV1().Secrets(ns).Update(ctx, sec, metav1.UpdateOptions{FieldManager: FieldManager})
		if err != nil {
			return "", err
		}
		return out.ResourceVersion, nil
	}
	return "", fmt.Errorf("data editing not supported for %s", resource)
}
