package pods

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Diagnosis is a plain-language explanation shown at the top of the panel.
type Diagnosis struct {
	Severity  string `json:"severity"` // error | warn | info
	Container string `json:"container"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	Hint      string `json:"hint"`
}

// exitMeaning explains a container exit in one line.
func exitMeaning(reason string, code int32) string {
	switch {
	case reason == "OOMKilled":
		return "morto por falta de memória (passou do limite)"
	case code == 0:
		return "terminou normalmente"
	case code == 137:
		return "SIGKILL: morto à força (kill -9)"
	case code == 143:
		return "SIGTERM: recebeu pedido de término"
	case code == 139:
		return "SIGSEGV: falha de segmentação"
	case code == 134:
		return "SIGABRT: processo abortou"
	case code == 126:
		return "comando sem permissão de execução"
	case code == 127:
		return "comando não encontrado"
	case code > 128:
		return fmt.Sprintf("encerrado pelo sinal %d", code-128)
	}
	return "a aplicação saiu com erro"
}

// Diagnose correlates container states, last terminations and the pod's
// events into "why is this pod restarting / not running" explanations.
func Diagnose(p *corev1.Pod, events []corev1.Event) []Diagnosis {
	var out []Diagnosis
	evs := sortEvents(events)

	// Pod-level problems first.
	if p.Status.Reason == "Evicted" {
		out = append(out, Diagnosis{Severity: "error", Title: "Pod despejado (Evicted)", Detail: p.Status.Message,
			Hint: "O node ficou sem recurso (memória, disco ou PIDs). Defina requests realistas e confira a pressão no node."})
	}
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse {
			out = append(out, Diagnosis{Severity: "error", Title: "Pod não foi agendado em nenhum node", Detail: c.Message,
				Hint: "Faltam recursos no cluster ou há nodeSelector/affinity/taints incompatíveis."})
		}
	}
	if e := latest(evs, func(e corev1.Event) bool { return e.Reason == "FailedMount" || e.Reason == "FailedAttachVolume" }); e != nil {
		out = append(out, Diagnosis{Severity: "error", Title: "Falha ao montar volume", Detail: e.Message,
			Hint: "Confira se o ConfigMap/Secret/PVC existe e se o volume não está preso a outro node."})
	}

	limits := map[string]resource.Quantity{}
	for _, c := range append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...) {
		if q, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			limits[c.Name] = q
		}
	}

	all := append(append([]corev1.ContainerStatus{}, p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...)
	for _, s := range all {
		ofContainer := func(e corev1.Event) bool {
			return strings.Contains(e.InvolvedObject.FieldPath, "{"+s.Name+"}")
		}

		if w := s.State.Waiting; w != nil {
			switch w.Reason {
			case "ImagePullBackOff", "ErrImagePull", "InvalidImageName", "ErrImageNeverPull":
				out = append(out, Diagnosis{Severity: "error", Container: s.Name, Title: "Não consegue baixar a imagem", Detail: w.Message,
					Hint: "Confira nome/tag da imagem e o acesso ao registry (imagePullSecrets ou integração do cluster com o ACR)."})
			case "CreateContainerConfigError", "CreateContainerError":
				out = append(out, Diagnosis{Severity: "error", Container: s.Name, Title: "Erro ao criar o container", Detail: w.Message,
					Hint: "Normalmente um ConfigMap/Secret referenciado (env ou volume) não existe ou falta uma chave."})
			}
		}

		crashLoop := s.State.Waiting != nil && s.State.Waiting.Reason == "CrashLoopBackOff"
		t := s.LastTerminationState.Terminated
		if t == nil && s.State.Terminated != nil && s.State.Terminated.ExitCode != 0 {
			t = s.State.Terminated
		}
		if t != nil && (s.RestartCount > 0 || crashLoop) {
			d := Diagnosis{Severity: "warn", Container: s.Name}
			if crashLoop {
				d.Severity = "error"
			}
			prefix := fmt.Sprintf("Reiniciou %d vez(es): ", s.RestartCount)
			if crashLoop {
				prefix = fmt.Sprintf("CrashLoopBackOff (%d reinícios): ", s.RestartCount)
			}

			killing := latest(evs, func(e corev1.Event) bool {
				return ofContainer(e) && e.Reason == "Killing" && strings.Contains(strings.ToLower(e.Message), "probe")
			})
			unhealthy := latest(evs, func(e corev1.Event) bool {
				return ofContainer(e) && e.Reason == "Unhealthy" && !strings.HasPrefix(e.Message, "Readiness")
			})

			switch {
			case t.Reason == "OOMKilled":
				d.Title = prefix + "falta de memória (OOMKilled)"
				if q, ok := limits[s.Name]; ok {
					d.Detail = fmt.Sprintf("O container passou do limite de memória (%s) e o kernel o matou.", q.String())
				} else {
					d.Detail = "O container não tem limite de memória e o node ficou sem memória; o kernel o matou."
				}
				d.Hint = "Aumente resources.limits.memory ou investigue o consumo (vazamento, cache, heap da JVM)."
			case killing != nil:
				d.Title = prefix + "a probe falhou e o kubelet reiniciou o container"
				d.Detail = killing.Message
				if unhealthy != nil {
					d.Detail += "\nÚltima falha: " + unhealthy.Message
				}
				d.Hint = "A aplicação não respondeu à probe a tempo. Confira o endpoint, ou ajuste initialDelaySeconds/timeoutSeconds/failureThreshold (ou use startupProbe se o boot é lento)."
			case t.ExitCode == 137:
				d.Title = prefix + "morto com SIGKILL (exit 137)"
				d.Detail = "O processo foi encerrado à força: kubelet, pressão de memória no node ou outro processo."
				d.Hint = "Veja os eventos do node e use 'Logs anteriores' para ver as últimas linhas antes da morte."
			case t.ExitCode == 0:
				d.Title = prefix + "o processo terminou sozinho (exit 0)"
				d.Detail = "O container encerrou sem erro, mas o restartPolicy o inicia de novo."
				d.Hint = "Containers de serviço não devem terminar: confira command/entrypoint. Para tarefas, use um Job."
			default:
				d.Title = fmt.Sprintf("%sexit %d (%s)", prefix, t.ExitCode, exitMeaning(t.Reason, t.ExitCode))
				d.Detail = strings.TrimSpace(t.Message)
				if d.Detail == "" {
					d.Detail = "A aplicação encerrou com erro."
				}
				d.Hint = "Abra 'Logs anteriores' deste container para ver o erro da execução que caiu."
			}
			if !t.FinishedAt.IsZero() {
				d.Detail = strings.TrimSpace(d.Detail + "\nÚltima queda: " + t.FinishedAt.UTC().Format("02/01 15:04:05 UTC"))
			}
			out = append(out, d)
		}

		if s.State.Running != nil && !s.Ready {
			if e := latest(evs, func(e corev1.Event) bool {
				return ofContainer(e) && e.Reason == "Unhealthy" && strings.HasPrefix(e.Message, "Readiness")
			}); e != nil {
				out = append(out, Diagnosis{Severity: "warn", Container: s.Name, Title: "Rodando, mas não está pronto (readiness falhando)",
					Detail: e.Message, Hint: "Enquanto não estiver pronto, o Service não envia tráfego para este pod."})
			}
		}
	}
	return out
}

func latest(evs []corev1.Event, match func(corev1.Event) bool) *corev1.Event {
	for i := range evs { // already newest-first
		if match(evs[i]) {
			return &evs[i]
		}
	}
	return nil
}
