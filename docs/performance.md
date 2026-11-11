# Performance methodology

The first storage baseline measures in-process Memory snapshot copy cost at1000 synthetic inventory entries. It does not measure PostgreSQL throughput, controller scalability or network service capacity. Run a fixed Go version and CPU budget:

```sh
GOMAXPROCS=2 go test ./tests -run '^$' -bench BenchmarkMemoryInventory1000 -benchmem -benchtime=2s -count=3
```

`tests/http_smoke.py` separately records30 sequential actual loopback HTTP snapshot requests backed by PostgreSQL, with inventory and audit sample size. Its median and nearest-rank p95 describe that tiny run only. The API token bucket remains enabled; no throughput extrapolation or concurrent deployment capacity is justified. Environment and raw run output are retained in evidence/performance.txt and evidence/http-smoke.json when executed.

Required future baseline: actual PostgreSQL read/write mix and row-lock contention; bounded event throughput once NATS is implemented; multi-controller reconciliation distributions; CPU/RSS over time; concurrent incident contention; failure injection and request-observed recovery duration. Use isolated test environments, reproducible generators, warmup/steady state, percentiles/error rates and raw result artifacts. Publish hardware, software versions, dataset/retention size, rate limits and confidence limitations. Never equate configured inventory caps with demonstrated capacity.
