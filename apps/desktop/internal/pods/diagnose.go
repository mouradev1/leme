package pods

import (
	"fmt"
	"leme/internal/platform/i18n"
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
		return i18n.T("morto por falta de memória (passou do limite)", "killed for lack of memory (over the limit)")
	case code == 0:
		return i18n.T("terminou normalmente", "exited normally")
	case code == 137:
		return i18n.T("SIGKILL: morto à força (kill -9)", "SIGKILL: forcibly killed (kill -9)")
	case code == 143:
		return i18n.T("SIGTERM: recebeu pedido de término", "SIGTERM: asked to terminate")
	case code == 139:
		return i18n.T("SIGSEGV: falha de segmentação", "SIGSEGV: segmentation fault")
	case code == 134:
		return i18n.T("SIGABRT: processo abortou", "SIGABRT: process aborted")
	case code == 126:
		return i18n.T("comando sem permissão de execução", "command is not executable")
	case code == 127:
		return i18n.T("comando não encontrado", "command not found")
	case code > 128:
		return fmt.Sprintf(i18n.T("encerrado pelo sinal %d", "terminated by signal %d"), code-128)
	}
	return i18n.T("a aplicação saiu com erro", "the application exited with an error")
}

// Diagnose correlates container states, last terminations and the pod's
// events into "why is this pod restarting / not running" explanations.
func Diagnose(p *corev1.Pod, events []corev1.Event) []Diagnosis {
	var out []Diagnosis
	evs := sortEvents(events)

	// Pod-level problems first.
	if p.Status.Reason == "Evicted" {
		out = append(out, Diagnosis{Severity: "error", Title: i18n.T("Pod despejado (Evicted)", "Pod evicted"), Detail: p.Status.Message,
			Hint: i18n.T("O node ficou sem recurso (memória, disco ou PIDs). Defina requests realistas e confira a pressão no node.", "The node ran out of resources (memory, disk or PIDs). Set realistic requests and check the node's pressure.")})
	}
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse {
			out = append(out, Diagnosis{Severity: "error", Title: i18n.T("Pod não foi agendado em nenhum node", "Pod was not scheduled on any node"), Detail: c.Message,
				Hint: i18n.T("Faltam recursos no cluster ou há nodeSelector/affinity/taints incompatíveis.", "The cluster lacks resources or nodeSelector/affinity/taints don't match.")})
		}
	}
	if e := latest(evs, func(e corev1.Event) bool { return e.Reason == "FailedMount" || e.Reason == "FailedAttachVolume" }); e != nil {
		out = append(out, Diagnosis{Severity: "error", Title: i18n.T("Falha ao montar volume", "Volume mount failed"), Detail: e.Message,
			Hint: i18n.T("Confira se o ConfigMap/Secret/PVC existe e se o volume não está preso a outro node.", "Check that the ConfigMap/Secret/PVC exists and the volume isn't attached to another node.")})
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
				out = append(out, Diagnosis{Severity: "error", Container: s.Name, Title: i18n.T("Não consegue baixar a imagem", "Can't pull the image"), Detail: w.Message,
					Hint: i18n.T("Confira nome/tag da imagem e o acesso ao registry (imagePullSecrets ou integração do cluster com o ACR).", "Check the image name/tag and registry access (imagePullSecrets or the cluster's registry integration).")})
			case "CreateContainerConfigError", "CreateContainerError":
				out = append(out, Diagnosis{Severity: "error", Container: s.Name, Title: i18n.T("Erro ao criar o container", "Error creating the container"), Detail: w.Message,
					Hint: i18n.T("Normalmente um ConfigMap/Secret referenciado (env ou volume) não existe ou falta uma chave.", "Usually a referenced ConfigMap/Secret (env or volume) is missing or lacks a key.")})
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
			prefix := fmt.Sprintf(i18n.T("Reiniciou %d vez(es): ", "Restarted %d time(s): "), s.RestartCount)
			if crashLoop {
				prefix = fmt.Sprintf(i18n.T("CrashLoopBackOff (%d reinícios): ", "CrashLoopBackOff (%d restarts): "), s.RestartCount)
			}

			killing := latest(evs, func(e corev1.Event) bool {
				return ofContainer(e) && e.Reason == "Killing" && strings.Contains(strings.ToLower(e.Message), "probe")
			})
			unhealthy := latest(evs, func(e corev1.Event) bool {
				return ofContainer(e) && e.Reason == "Unhealthy" && !strings.HasPrefix(e.Message, "Readiness")
			})

			switch {
			case t.Reason == "OOMKilled":
				d.Title = prefix + i18n.T("falta de memória (OOMKilled)", "out of memory (OOMKilled)")
				if q, ok := limits[s.Name]; ok {
					d.Detail = fmt.Sprintf(i18n.T("O container passou do limite de memória (%s) e o kernel o matou.", "The container went over its memory limit (%s) and the kernel killed it."), q.String())
				} else {
					d.Detail = i18n.T("O container não tem limite de memória e o node ficou sem memória; o kernel o matou.", "The container has no memory limit and the node ran out of memory; the kernel killed it.")
				}
				d.Hint = i18n.T("Aumente resources.limits.memory ou investigue o consumo (vazamento, cache, heap da JVM).", "Raise resources.limits.memory or look into the usage (leak, cache, JVM heap).")
			case killing != nil:
				d.Title = prefix + i18n.T("a probe falhou e o kubelet reiniciou o container", "the probe failed and the kubelet restarted the container")
				d.Detail = killing.Message
				if unhealthy != nil {
					d.Detail += i18n.T("\nÚltima falha: ", "\nLast failure: ") + unhealthy.Message
				}
				d.Hint = i18n.T("A aplicação não respondeu à probe a tempo. Confira o endpoint, ou ajuste initialDelaySeconds/timeoutSeconds/failureThreshold (ou use startupProbe se o boot é lento).", "The application didn't answer the probe in time. Check the endpoint, or tune initialDelaySeconds/timeoutSeconds/failureThreshold (or use a startupProbe if it boots slowly).")
			case t.ExitCode == 137:
				d.Title = prefix + i18n.T("morto com SIGKILL (exit 137)", "killed with SIGKILL (exit 137)")
				d.Detail = i18n.T("O processo foi encerrado à força: kubelet, pressão de memória no node ou outro processo.", "The process was forcibly killed: kubelet, node memory pressure or another process.")
				d.Hint = i18n.T("Veja os eventos do node e use 'Logs anteriores' para ver as últimas linhas antes da morte.", "Check the node's events and use 'Previous logs' to see the last lines before it died.")
			case t.ExitCode == 0:
				d.Title = prefix + i18n.T("o processo terminou sozinho (exit 0)", "the process exited on its own (exit 0)")
				d.Detail = i18n.T("O container encerrou sem erro, mas o restartPolicy o inicia de novo.", "The container exited without error, but the restartPolicy starts it again.")
				d.Hint = i18n.T("Containers de serviço não devem terminar: confira command/entrypoint. Para tarefas, use um Job.", "Service containers should not exit: check command/entrypoint. For one-off tasks, use a Job.")
			default:
				d.Title = fmt.Sprintf("%sexit %d (%s)", prefix, t.ExitCode, exitMeaning(t.Reason, t.ExitCode))
				d.Detail = strings.TrimSpace(t.Message)
				if d.Detail == "" {
					d.Detail = i18n.T("A aplicação encerrou com erro.", "The application exited with an error.")
				}
				d.Hint = i18n.T("Abra 'Logs anteriores' deste container para ver o erro da execução que caiu.", "Open this container's 'Previous logs' to see the error of the run that crashed.")
			}
			if !t.FinishedAt.IsZero() {
				d.Detail = strings.TrimSpace(d.Detail + i18n.T("\nÚltima queda: ", "\nLast crash: ") + t.FinishedAt.UTC().Format("02/01 15:04:05 UTC"))
			}
			out = append(out, d)
		}

		if s.State.Running != nil && !s.Ready {
			if e := latest(evs, func(e corev1.Event) bool {
				return ofContainer(e) && e.Reason == "Unhealthy" && strings.HasPrefix(e.Message, "Readiness")
			}); e != nil {
				out = append(out, Diagnosis{Severity: "warn", Container: s.Name, Title: i18n.T("Rodando, mas não está pronto (readiness falhando)", "Running but not ready (readiness failing)"),
					Detail: e.Message, Hint: i18n.T("Enquanto não estiver pronto, o Service não envia tráfego para este pod.", "Until it is ready, the Service sends no traffic to this pod.")})
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
