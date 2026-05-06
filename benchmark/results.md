## Benchmark Results

### Methodology

All benchmarks use [hyperfine](https://github.com/sharkdp/hyperfine) with 3 warmup runs
and 10 timed runs per benchmark. Times shown are mean ± σ wall-clock time. Peak memory is
measured via `/usr/bin/time -l` (median of 3 runs).

Benchmark files are valid Clojure that runs unmodified on let-go, babashka, joker, glojure,
and Clojure JVM. Fennel uses equivalent implementations via
[fennel-cljlib](https://gitlab.com/andreyorst/fennel-cljlib) (lazy seqs, transducers,
persistent data structures). Gloat benchmarks are pre-compiled to native binaries via
[gloat](https://github.com/gloathub/gloat) AOT (Clojure→Go); compilation time is not
measured, only binary execution (analogous to how let-go is pre-built with `go build`).

Clojure JVM times include full JVM startup (~350-500ms) which dominates short benchmarks.
Joker is skipped for benchmarks that would exceed reasonable time limits or use unsupported
features (transducers). Binary sizes for gloat are averaged across all benchmark binaries.

**System:** Darwin arm64, Apple M1 Pro

**Runtimes:**

| | let-go | babashka | joker | fennel | clojure JVM |
|---|---|---|---|---|---|
| **Version** | — | babashka v1.12.217 | joker v1.7.1 | — | Fennel 1.6.1 on PUC Lua 5.5 | Clojure CLI version 1.12.4.1618 |
| **Platform** | Go bytecode VM | GraalVM native | Go tree-walk interpreter | Go AOT (Clojure→Go) | Lua VM + cljlib | JVM (HotSpot) |
| **Binary/runtime size** | **10M** | 68M | 26M | — | 324K | 304M |

### Startup Time

| Runtime | Time |
|---|---|
| **let-go** | **6.9ms ± 0.5ms** (1.0x) |
| babashka | 20.4ms ± 0.9ms (2.9x) |
| joker | 11.8ms ± 0.6ms (1.7x) |
| fennel | 50.2ms ± 5.3ms (7.2x) |
| clojure JVM | 0.331s ± 0.007s (47.8x) |

### Peak Memory Usage (RSS)

| Workload | let-go | babashka | joker | fennel | clojure JVM |
|---|---|---|---|---|---|
| startup (nil) | 13.6MB (1.0x) | 26.8MB (2.0x) | 21.3MB (1.6x) | **3.1MB** (0.2x) | 92.2MB (6.8x) |
| fib(35) | 14.4MB (1.0x) | 77.1MB (5.4x) | 33.5MB (2.3x) | **12.8MB** (0.9x) | 111.5MB (7.7x) |
| reduce 1M | **20.0MB** (1.0x) | 59.0MB (3.0x) | 32.7MB (1.6x) | 885.7MB (44.3x) | 117.5MB (5.9x) |

### Performance

| Benchmark | let-go | babashka | joker | fennel | clojure JVM |
|---|---|---|---|---|---|
| fib | 1.980s ± 0.034s (1.0x) | 1.897s ± 0.029s (1.0x) | 19.468s ± 0.180s (9.8x) | 1.976s ± 0.083s (1.0x) | **0.539s ± 0.008s** (0.3x) |
| loop-recur | **58.5ms ± 0.6ms** (1.0x) | 0.104s ± 0.100s (1.8x) | 0.702s ± 0.013s (12.0x) | 0.175s ± 0.004s (3.0x) | 0.453s ± 0.015s (7.7x) |
| map-filter | **7.9ms ± 0.7ms** (1.0x) | 18.6ms ± 1.7ms (2.4x) | 13.1ms ± 1.2ms (1.7x) | 1.020s ± 0.030s (129.2x) | 0.353s ± 0.014s (44.7x) |
| persistent-map | **19.3ms ± 0.9ms** (1.0x) | 23.1ms ± 2.9ms (1.2x) | 49.2ms ± 1.5ms (2.5x) | 3.598s ± 0.050s (185.9x) | 0.471s ± 0.009s (24.3x) |
| reduce | 72.9ms ± 1.6ms (1.0x) | **36.9ms ± 5.5ms** (0.5x) | 2.472s ± 0.033s (33.9x) | 7.865s ± 0.098s (107.9x) | 0.339s ± 0.008s (4.7x) |
| tak | 2.030s ± 0.033s (1.0x) | 1.897s ± 0.038s (0.9x) | — | 10.542s ± 0.123s (5.2x) | **0.576s ± 0.026s** (0.3x) |
| transducers | **8.1ms ± 0.2ms** (1.0x) | 19.2ms ± 1.6ms (2.4x) | — | 0.998s ± 0.029s (123.7x) | 0.344s ± 0.014s (42.6x) |

