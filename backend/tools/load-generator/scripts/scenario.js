// Fleet scenario for the ceiling campaign (load-testing-plan.md §7).
//
// The externally-controlled executor starts at 0 VUs; the run orchestrator
// (tools/load-generator/runner) scales VUs over the k6 REST API, so ramp
// steps keep existing connections alive. Each VU holds one fleet of
// PODS_PER_VU virtual dumpers until it is scaled away: 1 pod per VU for the
// T2 throughput runs, ~100 idle pods per VU for the T3 connection runs.
//
// Every workload knob (§4) comes from env so the frozen run spec is the
// single source of load-shape truth. Explicit zeros are honored — T3 sets
// THREADS_PER_POD=0 for keep-alive-only pods.
import cdt from 'k6/x/cdt';

function num(name, dflt) {
    const v = __ENV[name];
    return v === undefined || v === '' ? dflt : Number(v);
}

function str(name, dflt) {
    const v = __ENV[name];
    return v === undefined || v === '' ? dflt : v;
}

export const options = {
    scenarios: {
        fleet: {
            executor: 'externally-controlled',
            vus: 0,
            maxVUs: num('MAX_VUS', 600),
            duration: str('DURATION', '2h'),
        },
    },
    // The run label: keeps this run's series apart in VictoriaMetrics
    // (doc/run-orchestration.md).
    tags: { testid: str('TESTID', 'dev') },
};

export default function () {
    const summary = cdt.runFleet({
        addr: `${str('COLLECTOR_HOST', 'localhost')}:${num('COLLECTOR_PORT', 1715)}`,
        pods: num('PODS_PER_VU', 1),
        namespace: str('EMULATOR_NAMESPACE', 'load'),
        service: str('EMULATOR_SERVICE', 'load-svc'),
        podPrefix: str('EMULATOR_POD_PREFIX', ''),
        seed: num('SEED', 1),
        startSpread: str('START_SPREAD', '2s'),

        threadsPerPod: num('THREADS_PER_POD', 8),
        callsPerSec: num('CALLS_PER_SEC', 5),
        dictInitial: num('DICT_INITIAL', 2000),
        dictGrowthPerMin: num('DICT_GROWTH_PER_MIN', 10),
        durationThresholds: str('DURATION_THRESHOLDS', '100ms,1s,10s'),
        durationShares: str('DURATION_SHARES', '0.90,0.07,0.025,0.005'),
        stackDepth: num('STACK_DEPTH', 10),
        sqlShare: num('SQL_SHARE', 0.2),
        sqlBytes: num('SQL_BYTES', 1024),
        sqlDedup: num('SQL_DEDUP', 0.9),
        xmlShare: num('XML_SHARE', 0.05),
        xmlBytes: num('XML_BYTES', 4096),
        suspendRate: num('SUSPEND_RATE', 0.5),
        errorShare: num('ERROR_SHARE', 0.01),
        requestIdShare: num('REQUEST_ID_SHARE', 1),
        cpuFraction: num('CPU_FRACTION', 0),
        waitFraction: num('WAIT_FRACTION', 0),
        memoryBytes: num('MEMORY_BYTES', 4096),
    });
    console.log(`fleet done: ${JSON.stringify(summary)}`);
}
