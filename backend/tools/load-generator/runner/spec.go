package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Spec is the run specification of doc/run-orchestration.md: everything a
// ceiling run needs, frozen into the artifact directory so runs stay
// comparable.
type Spec struct {
	Run struct {
		Name   string `yaml:"name"`
		TestID string `yaml:"testid"`
	} `yaml:"run"`
	Outputs string `yaml:"outputs"`

	Endpoints struct {
		K6        string `yaml:"k6"`
		VM        string `yaml:"vm"`
		Collector string `yaml:"collector"`
	} `yaml:"endpoints"`

	Images     map[string]string `yaml:"images"`
	HelmValues string            `yaml:"helmValues"`
	Workload   map[string]string `yaml:"workload"`

	Ramp struct {
		Levels  []int `yaml:"levels"`
		Confirm struct {
			Timeout          duration `yaml:"timeout"`
			ConnectionsPerVU int      `yaml:"connectionsPerVU"`
			// ConnectionsQuery overrides the default active-connections
			// query when connectionsPerVU > 0.
			ConnectionsQuery string `yaml:"connectionsQuery"`
		} `yaml:"confirm"`
		Hold struct {
			Min     duration `yaml:"min"`
			Max     duration `yaml:"max"`
			Sample  duration `yaml:"sample"`
			Plateau struct {
				Window         duration          `yaml:"window"`
				SlopeTolerance float64           `yaml:"slopeTolerance"`
				Series         map[string]string `yaml:"series"`
			} `yaml:"plateau"`
		} `yaml:"hold"`
	} `yaml:"ramp"`

	Detectors []Detector        `yaml:"detectors"`
	Context   map[string]string `yaml:"context"`

	Guard struct {
		GeneratorCPU struct {
			Query      string  `yaml:"query"`
			LimitCores float64 `yaml:"limitCores"`
			MaxShare   float64 `yaml:"maxShare"`
		} `yaml:"generator-cpu"`
	} `yaml:"guard"`

	Pprof struct {
		Points   []float64 `yaml:"points"`
		Seconds  int       `yaml:"seconds"`
		Profiles []string  `yaml:"profiles"`
	} `yaml:"pprof"`
}

// Detector is one saturation signal; kinds are defined in
// doc/run-orchestration.md.
type Detector struct {
	Name  string `yaml:"name"`
	Kind  string `yaml:"kind"`
	Query string `yaml:"query"`
	// Share is the sticky-share trigger fraction.
	Share float64 `yaml:"share"`
	// MinGrowth is the monotonic-growth relative trigger.
	MinGrowth float64 `yaml:"minGrowth"`
	// MinValue is an absolute floor for monotonic-growth: the detector stays
	// silent while the last sample is below it. A gauge that oscillates down
	// to zero (pending-parquet bytes between upload cycles) otherwise reads
	// every rising sawtooth edge as growth from zero.
	MinValue float64 `yaml:"minValue"`
	// Ratio is the baseline-ratio trigger multiple.
	Ratio float64 `yaml:"ratio"`
	// Grace excludes the first samples of every hold from this detector: a
	// cold start fills empty stores, and growth-shaped detectors would read
	// that fill as saturation (doc/run-orchestration.md).
	Grace duration `yaml:"grace"`
}

// duration wraps time.Duration for YAML ("3m", "15s").
type duration time.Duration

func (d *duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = duration(v)
	return nil
}

func (d duration) std() time.Duration { return time.Duration(d) }

var detectorKinds = map[string]bool{
	"sticky-share": true, "monotonic-growth": true, "nonzero": true, "baseline-ratio": true,
}

// LoadSpec reads and validates a run spec.
func LoadSpec(path string) (*Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &s, s.validate()
}

func (s *Spec) validate() error {
	switch {
	case s.Run.Name == "":
		return fmt.Errorf("run.name is required")
	case s.Run.TestID == "":
		return fmt.Errorf("run.testid is required — the unique run label keeps VM series of different runs apart")
	case s.Endpoints.K6 == "" || s.Endpoints.VM == "" || s.Endpoints.Collector == "":
		return fmt.Errorf("endpoints.k6, endpoints.vm, and endpoints.collector are all required")
	case len(s.Ramp.Levels) == 0:
		return fmt.Errorf("ramp.levels must list at least one VU step")
	case len(s.Ramp.Hold.Plateau.Series) == 0:
		return fmt.Errorf("ramp.hold.plateau.series must name at least one series (what would the hold wait for?)")
	}
	prev := 0
	for _, l := range s.Ramp.Levels {
		if l <= prev {
			return fmt.Errorf("ramp.levels must increase strictly, got %v", s.Ramp.Levels)
		}
		prev = l
	}
	for _, d := range s.Detectors {
		if !detectorKinds[d.Kind] {
			return fmt.Errorf("detector %q: unknown kind %q", d.Name, d.Kind)
		}
		if d.Query == "" {
			return fmt.Errorf("detector %q: query is required", d.Name)
		}
	}
	if s.Outputs == "" {
		s.Outputs = "runs"
	}
	if s.Ramp.Confirm.Timeout == 0 {
		s.Ramp.Confirm.Timeout = duration(3 * time.Minute)
	}
	if s.Ramp.Hold.Min == 0 {
		s.Ramp.Hold.Min = duration(3 * time.Minute)
	}
	if s.Ramp.Hold.Max == 0 {
		s.Ramp.Hold.Max = duration(15 * time.Minute)
	}
	if s.Ramp.Hold.Sample == 0 {
		s.Ramp.Hold.Sample = duration(15 * time.Second)
	}
	if s.Ramp.Hold.Plateau.Window == 0 {
		s.Ramp.Hold.Plateau.Window = duration(2 * time.Minute)
	}
	if s.Ramp.Hold.Plateau.SlopeTolerance == 0 {
		s.Ramp.Hold.Plateau.SlopeTolerance = 0.05
	}
	if len(s.Pprof.Points) == 0 {
		s.Pprof.Points = []float64{0.7, 1.0}
	}
	if s.Pprof.Seconds == 0 {
		s.Pprof.Seconds = 30
	}
	if len(s.Pprof.Profiles) == 0 {
		s.Pprof.Profiles = []string{"profile", "heap", "goroutine"}
	}
	return nil
}
