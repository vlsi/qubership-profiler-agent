package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// podInfo is the slice of pod status §8.8 needs: identity plus the summed
// container restart count.
type podInfo struct {
	Name     string
	UID      string
	Restarts int
}

// podLister lists the backend pods under watch. The production
// implementation talks to the k8s API; tests use a fake.
type podLister interface {
	list(ctx context.Context) ([]podInfo, error)
}

// kubeLister resolves the client the way tools/migration does: kubeconfig
// (KUBECONFIG or the default home path) first, in-cluster config as the
// fallback for a checker running as a pod.
type kubeLister struct {
	client    kubernetes.Interface
	namespace string
	selector  string
}

func newKubeLister(namespace, selector string) (*kubeLister, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("no kubeconfig and no in-cluster config: %w", err)
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &kubeLister{client: client, namespace: namespace, selector: selector}, nil
}

func (k *kubeLister) list(ctx context.Context) ([]podInfo, error) {
	pods, err := k.client.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{LabelSelector: k.selector})
	if err != nil {
		return nil, err
	}
	out := make([]podInfo, 0, len(pods.Items))
	for _, p := range pods.Items {
		restarts := 0
		for _, cs := range p.Status.ContainerStatuses {
			restarts += int(cs.RestartCount)
		}
		out = append(out, podInfo{Name: p.Name, UID: string(p.UID), Restarts: restarts})
	}
	return out, nil
}

// podTrack follows one pod UID across the run.
type podTrack struct {
	name string
	// baseline is the restart count at first sight: the first successful
	// list for baseline pods, zero for replacements (their own restarts all
	// count, plus the +1 replacement event).
	baseline      int
	last          int
	alive         bool
	isReplacement bool
}

// podState is the §8.8 accounting (doc/checker.md): a total restart budget
// over the whole run, replacements counted as one event plus their own
// restarts, disappearances without replacement flagged separately.
type podState struct {
	allowed int

	baselined    bool
	tracks       map[string]*podTrack
	replacements int
	gone         []string // names of pods that vanished, in vanish order
	current      []finding
}

func newPodState(allowed int) *podState {
	return &podState{allowed: allowed, tracks: map[string]*podTrack{}}
}

func (p *podState) findings() []finding { return p.current }

// observe folds one successful pod list into the accounting and refreshes
// the invariant findings.
func (p *podState) observe(pods []podInfo) {
	seen := map[string]bool{}
	for _, pod := range pods {
		seen[pod.UID] = true
		track, ok := p.tracks[pod.UID]
		if !ok {
			track = &podTrack{name: pod.Name, isReplacement: p.baselined}
			if !p.baselined {
				track.baseline = pod.Restarts
			}
			p.tracks[pod.UID] = track
			if p.baselined {
				p.replacements++
			}
		}
		track.last = pod.Restarts
		track.alive = true
	}
	for uid, track := range p.tracks {
		if track.alive && !seen[uid] {
			track.alive = false
			p.gone = append(p.gone, track.name)
		}
	}
	p.baselined = true
	p.current = p.evaluate()
}

func (p *podState) evaluate() []finding {
	var out []finding
	budget := 0
	var contributors []string
	for _, track := range p.tracks {
		if delta := track.last - track.baseline; delta > 0 {
			budget += delta
			contributors = append(contributors, fmt.Sprintf("%s +%d", track.name, delta))
		}
		if track.isReplacement {
			contributors = append(contributors, track.name+" (replacement)")
		}
	}
	budget += p.replacements
	if budget > p.allowed {
		sort.Strings(contributors)
		out = append(out, finding{subject: "restart-budget",
			msg: fmt.Sprintf("%d restart events exceed the budget of %d: %s",
				budget, p.allowed, strings.Join(contributors, ", "))})
	}
	if excess := len(p.gone) - p.replacements; excess > 0 {
		names := p.gone[len(p.gone)-excess:]
		out = append(out, finding{subject: "pods-gone",
			msg: fmt.Sprintf("%d pod(s) disappeared without a replacement: %s", excess, strings.Join(names, ", "))})
	}
	return out
}
