# Load-testing stand

Helmfile composition of the static stand from
[the load-testing plan](../../../docs/design/load-testing-plan.md) §5.3: monitoring CRDs +
[qubership-monitoring-operator](https://github.com/Netcracker/qubership-monitoring-operator), MinIO as the S3 target,
the profiler backend under test, the stand's Grafana dashboards (`monitoring-crs`), and the k6 runner. Run
orchestration (scenario parameters, ramp steps, fault injection, artifact collection) is a separate script layer and
does not live here.

## Prerequisites

- `helm` and `helmfile` (v1+); the monitoring-operator charts come straight from their git tag through helmfile's
  built-in `git::` chart support, no helm plugin needed.
- A kubectl context pointing at the target cluster (OrbStack or kind locally).
- Images loadable by the cluster:

```bash
# The backend under test (the Makefile tags it latest; the chart pulls dev):
make -C backend/apps/profiler-backend docker-build
docker tag profiler-backend:latest profiler-backend:dev
# The k6 runner — needs prepared data/ (doc/preparing_data.md); parked until
# the phase-2 virtual dumper, see k6.installed in environments/:
docker build -t cdt-load-generator:dev backend/tools/load-generator
# kind only — OrbStack shares the host docker images:
kind load docker-image profiler-backend:dev cdt-load-generator:dev
```

## Usage

```bash
cd backend/tools/load-generator/deploy
helmfile -e local apply      # local stand (also the default environment)
helmfile -e cluster apply    # large cluster; set storage classes and image refs first
```

The `k6` release starts sending load as soon as its pod is up. To bring the stand up without load, set
`k6.installed: false` in the environment file (or `--state-values-set k6.installed=false`), and flip it back for the
run.

Reaching the UIs from the local stand:

```bash
kubectl -n monitoring port-forward svc/grafana-service 3000:3000     # Grafana
kubectl -n profiler-load port-forward svc/profiler-backend-query 8080:8080  # backend UI + /api/v1
kubectl -n profiler-load port-forward svc/minio-console 9001:9001    # MinIO console
```

Grafana admin credentials live in the `grafana-admin-credentials` secret of the monitoring namespace.

## pprof

`PROFILER_PPROF_ENABLED=true` is set on all three backend workloads. Take a profile through a port-forward of the
internal port:

```bash
kubectl -n profiler-load port-forward profiler-backend-collector-0 8081:8081
go tool pprof http://localhost:8081/debug/pprof/profile?seconds=30
```

## Layout

```text
helmfile.yaml            releases + needs ordering (CRDs -> operator -> CRs)
environments/            local vs cluster value layers (sizes, replicas, classes)
values/*.yaml.gotmpl     per-release values, parameterized by the environment
charts/monitoring-crs/   wraps ../dashboards/*.json into GrafanaDashboard CRs
```
