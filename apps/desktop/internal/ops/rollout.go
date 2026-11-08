package ops

import (
	"context"
	"errors"
	"fmt"
	"leme/internal/platform/i18n"
	"slices"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"leme/internal/cluster"
)

const revisionAnno = "deployment.kubernetes.io/revision"
const changeCauseAnno = "kubernetes.io/change-cause"

// Rollout is a Deployment's progress and its revision history.
type Rollout struct {
	Desired   int32      `json:"desired"`
	Updated   int32      `json:"updated"`
	Ready     int32      `json:"ready"`
	Available int32      `json:"available"`
	Revision  int64      `json:"revision"`
	Done      bool       `json:"done"`
	Failed    bool       `json:"failed"` // progress deadline exceeded
	Paused    bool       `json:"paused"`
	Message   string     `json:"message"`
	History   []Revision `json:"history"` // newest first
}

type Revision struct {
	Revision    int64    `json:"revision"`
	Images      []string `json:"images"`
	ChangeCause string   `json:"changeCause"`
	Created     string   `json:"created"` // RFC 3339
	Replicas    int32    `json:"replicas"`
	Current     bool     `json:"current"`
}

func revisionOf(m metav1.Object) int64 {
	v, _ := strconv.ParseInt(m.GetAnnotations()[revisionAnno], 10, 64)
	return v
}

// ownedReplicaSets returns the Deployment's ReplicaSets (by owner reference).
func ownedReplicaSets(ctx context.Context, kube kubernetes.Interface, d *appsv1.Deployment) ([]appsv1.ReplicaSet, error) {
	sel, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, err
	}
	list, err := kube.AppsV1().ReplicaSets(d.Namespace).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return nil, err
	}
	var out []appsv1.ReplicaSet
	for _, rs := range list.Items {
		for _, o := range rs.OwnerReferences {
			if o.UID == d.UID && o.Controller != nil && *o.Controller {
				out = append(out, rs)
				break
			}
		}
	}
	return out, nil
}

// RolloutStatus reads progress the way `kubectl rollout status` does.
func RolloutStatus(ctx context.Context, s *cluster.Session, ns, name string) (Rollout, error) {
	d, err := s.Kube.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return Rollout{}, err
	}
	r := Rollout{
		Desired: 1, Updated: d.Status.UpdatedReplicas, Ready: d.Status.ReadyReplicas,
		Available: d.Status.AvailableReplicas, Revision: revisionOf(d), Paused: d.Spec.Paused,
	}
	if d.Spec.Replicas != nil {
		r.Desired = *d.Spec.Replicas
	}
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentProgressing && c.Reason == "ProgressDeadlineExceeded" {
			r.Failed = true
			r.Message = c.Message
		}
	}
	switch {
	case r.Failed:
	case d.Generation > d.Status.ObservedGeneration:
		r.Message = i18n.T("aguardando o controlador ver a mudança", "waiting for the controller to see the change")
	case r.Updated < r.Desired:
		r.Message = fmt.Sprintf(i18n.T("%d de %d pods atualizados", "%d of %d pods updated"), r.Updated, r.Desired)
	case d.Status.Replicas > r.Updated:
		r.Message = fmt.Sprintf(i18n.T("encerrando %d pod(s) da versão antiga", "terminating %d old pod(s)"), d.Status.Replicas-r.Updated)
	case r.Available < r.Updated:
		r.Message = fmt.Sprintf(i18n.T("%d de %d pods novos disponíveis", "%d of %d new pods available"), r.Available, r.Updated)
	default:
		r.Done = true
		r.Message = i18n.T("rollout concluído", "rollout complete")
	}
	if r.Paused {
		r.Message = i18n.T("rollout pausado", "rollout paused")
	}

	rss, err := ownedReplicaSets(ctx, s.Kube, d)
	if err != nil {
		return r, nil // history is a bonus: status still useful without list permission
	}
	for _, rs := range rss {
		rev := Revision{
			Revision: revisionOf(&rs), ChangeCause: rs.Annotations[changeCauseAnno],
			Created: rs.CreationTimestamp.UTC().Format(time.RFC3339), Replicas: rs.Status.Replicas,
		}
		for _, c := range rs.Spec.Template.Spec.Containers {
			rev.Images = append(rev.Images, c.Image)
		}
		rev.Current = rev.Revision == r.Revision
		r.History = append(r.History, rev)
	}
	slices.SortFunc(r.History, func(a, b Revision) int { return int(b.Revision - a.Revision) })
	return r, nil
}

// RolloutUndo rolls a Deployment back to a revision (0 = the previous one),
// like `kubectl rollout undo`: the old ReplicaSet's pod template is copied
// back into the Deployment, which then rolls out as usual.
func RolloutUndo(ctx context.Context, s *cluster.Session, ns, name string, revision int64) (int64, error) {
	d, err := s.Kube.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}
	if d.Spec.Paused {
		return 0, fmt.Errorf(i18n.T("o rollout de %s está pausado: retome antes de voltar a versão", "the rollout of %s is paused: resume it before rolling back"), name)
	}
	rss, err := ownedReplicaSets(ctx, s.Kube, d)
	if err != nil {
		return 0, err
	}
	current := revisionOf(d)
	var target *appsv1.ReplicaSet
	for i := range rss {
		rev := revisionOf(&rss[i])
		switch {
		case revision > 0 && rev == revision:
			target = &rss[i]
		case revision == 0 && rev < current && (target == nil || rev > revisionOf(target)):
			target = &rss[i] // highest revision below the current one
		}
	}
	if target == nil {
		if revision == 0 {
			return 0, fmt.Errorf(i18n.T("não há versão anterior de %s para voltar", "there is no previous version of %s to roll back to"), name)
		}
		return 0, fmt.Errorf(i18n.T("a revisão %d de %s não existe mais (o histórico guarda poucas versões)", "revision %d of %s no longer exists (history keeps only a few versions)"), revision, name)
	}
	if revisionOf(target) == current {
		return 0, fmt.Errorf(i18n.T("a revisão %d já é a atual", "revision %d is already the current one"), current)
	}
	tpl := target.Spec.Template.DeepCopy()
	delete(tpl.Labels, appsv1.DefaultDeploymentUniqueLabelKey) // pod-template-hash belongs to the ReplicaSet
	d.Spec.Template = *tpl
	if d.Annotations == nil {
		d.Annotations = map[string]string{}
	}
	if cause := target.Annotations[changeCauseAnno]; cause != "" {
		d.Annotations[changeCauseAnno] = cause
	} else {
		delete(d.Annotations, changeCauseAnno)
	}
	if _, err := s.Kube.AppsV1().Deployments(ns).Update(ctx, d, metav1.UpdateOptions{FieldManager: "leme"}); err != nil {
		if strings.Contains(err.Error(), "the object has been modified") {
			return 0, errors.New(i18n.T(i18n.T("o deployment mudou enquanto voltava a versão: tente de novo", "the deployment changed during the rollback: try again"), "the deployment changed during the rollback: try again"))
		}
		return 0, err
	}
	return revisionOf(target), nil
}
