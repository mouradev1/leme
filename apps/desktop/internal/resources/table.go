package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// Asking for meta.k8s.io/v1 Table makes the API server render the same
// columns `kubectl get` shows, for any kind, including CRD
// additionalPrinterColumns. With includeObject=Metadata each row carries only
// ObjectMeta, never spec/status: the payload is a fraction of a full list, and
// no Go types are needed per kind.
const tableAccept = "application/json;as=Table;v=v1;g=meta.k8s.io,application/json"

type Query struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	FieldSelector string `json:"fieldSelector"`
}

type Column struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Format      string `json:"format"`
	Description string `json:"description"`
	Priority    int32  `json:"priority"` // >0 = "wide" column, hidden by default
}

// OwnerRef is the controlling owner (e.g. Pod -> ReplicaSet), shown as a
// clickable "Controlled By" column for any kind.
type OwnerRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type Row struct {
	UID       string      `json:"uid"`
	Name      string      `json:"name"`
	Namespace string      `json:"namespace,omitempty"`
	Created   metav1.Time `json:"created"` // lets the UI render a live "Age"
	RV        string      `json:"rv"`
	Cells     []any       `json:"cells"`
	// Labels make "app=nginx" searchable client-side. Annotations are left
	// out on purpose: last-applied-configuration alone can be kilobytes.
	Labels map[string]string `json:"labels,omitempty"`
	Owner  *OwnerRef         `json:"owner,omitempty"`
}

type Page struct {
	Columns  []Column
	Rows     []Row
	RV       string
	Continue string
}

func tableRequest(rc rest.Interface, ref Ref, q Query) *rest.Request {
	req := rc.Get().AbsPath(ref.Path(q.Namespace)).
		SetHeader("Accept", tableAccept).
		Param("includeObject", "Metadata")
	if q.LabelSelector != "" {
		req = req.Param("labelSelector", q.LabelSelector)
	}
	if q.FieldSelector != "" {
		req = req.Param("fieldSelector", q.FieldSelector)
	}
	return req
}

// ListPage fetches one page (limit/continue) so huge clusters render
// progressively instead of blocking on a 50 MB response.
func ListPage(ctx context.Context, rc rest.Interface, ref Ref, q Query, cont string, limit int64) (*Page, error) {
	req := tableRequest(rc, ref, q).Param("limit", strconv.FormatInt(limit, 10))
	if cont != "" {
		req = req.Param("continue", cont)
	}
	raw, err := req.Do(ctx).Raw()
	if err != nil {
		return nil, err
	}
	t, err := decodeTable(raw)
	if err != nil {
		return nil, err
	}
	p := &Page{Rows: rowsOf(t), RV: t.ResourceVersion, Continue: t.Continue}
	for _, c := range t.ColumnDefinitions {
		p.Columns = append(p.Columns, Column{c.Name, c.Type, c.Format, c.Description, c.Priority})
	}
	return p, nil
}

// OpenWatch starts a Table-formatted watch from rv. The server closes it
// after timeoutSeconds; callers re-open from the last seen resourceVersion.
func OpenWatch(ctx context.Context, rc rest.Interface, ref Ref, q Query, rv string) (io.ReadCloser, error) {
	timeout := 240 + rand.IntN(120) // jitter so many views don't reconnect in lockstep
	return tableRequest(rc, ref, q).
		Param("watch", "true").
		Param("resourceVersion", rv).
		Param("timeoutSeconds", strconv.Itoa(timeout)).
		Stream(ctx)
}

// WatchEvent is one frame of a watch stream; Object is a Table with 1 row
// (or a metav1.Status when Type == ERROR).
type WatchEvent struct {
	Type   string          `json:"type"`
	Object json.RawMessage `json:"object"`
}

func DecodeRows(raw []byte) ([]Row, error) {
	t, err := decodeTable(raw)
	if err != nil {
		return nil, err
	}
	return rowsOf(t), nil
}

func decodeTable(raw []byte) (*metav1.Table, error) {
	var t metav1.Table
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, err
	}
	if t.Kind != "Table" {
		return nil, fmt.Errorf("server did not return a Table (got %q)", t.Kind)
	}
	return &t, nil
}

func rowsOf(t *metav1.Table) []Row {
	rows := make([]Row, 0, len(t.Rows))
	for _, r := range t.Rows {
		var m metav1.PartialObjectMetadata
		if len(r.Object.Raw) > 0 {
			_ = json.Unmarshal(r.Object.Raw, &m)
		}
		row := Row{
			UID: string(m.UID), Name: m.Name, Namespace: m.Namespace,
			Created: m.CreationTimestamp, RV: m.ResourceVersion, Cells: r.Cells, Labels: m.Labels,
		}
		if o := metav1.GetControllerOfNoCopy(&m); o != nil {
			row.Owner = &OwnerRef{Kind: o.Kind, Name: o.Name}
		} else if len(m.OwnerReferences) > 0 {
			row.Owner = &OwnerRef{Kind: m.OwnerReferences[0].Kind, Name: m.OwnerReferences[0].Name}
		}
		rows = append(rows, row)
	}
	return rows
}
