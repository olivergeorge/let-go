## Benchmark Results

### Methodology

All benchmarks use [hyperfine](https://github.com/sharkdp/hyperfine) with 3 warmup runs
and 10 timed runs per benchmark. Times shown are mean ± σ wall-clock time. Each benchmark
file is valid Clojure that runs unmodified on all runtimes (excluding fennel). Peak memory is measured
via `/usr/bin/time -l` (median of 3 runs). Clojure JVM times include full JVM startup
(~350-500ms) which dominates short benchmarks. Joker is skipped for benchmarks that
would exceed reasonable time limits or use unsupported features (transducers).

**System:** Darwin arm64, Apple M1 Pro

**Runtimes:**

|                         | let-go         | babashka           | joker                    | glojure                                      | fennel                      | clojure JVM                     |
| ----------------------- | -------------- | ------------------ | ------------------------ | -------------------------------------------- | --------------------------- | ------------------------------- |
| **Version**             | —              | babashka v1.12.217 | joker v1.7.1             | glojure v0.6.5-0.20260329182937-9eacc0f13cb4 | Fennel 1.6.1 on PUC Lua 5.5 | Clojure CLI version 1.12.4.1618 |
| **Platform**            | Go bytecode VM | GraalVM native     | Go tree-walk interpreter | Go tree-walk interpreter                     | Lua VM + cljlib             | JVM (HotSpot)                   |
| **Binary/runtime size** | **9.6M**       | 68M                | 26M                      | 57M                                          | 324K                        | 304M                            |

### Startup Time

| Runtime     | Time                     |
| ----------- | ------------------------ |
| **let-go**  | **6.5ms ± 0.6ms** (1.0x) |
| babashka    | 20.0ms ± 1.8ms (3.1x)    |
| joker       | 12.9ms ± 1.8ms (2.0x)    |
| glojure     | 43.7ms ± 1.5ms (6.7x)    |
| fennel      | 43.1ms ± 10.8ms (6.6x)   |
| clojure JVM | 0.322s ± 0.008s (49.5x)  |

### Peak Memory Usage (RSS)

| Workload      | let-go            | babashka      | joker         | glojure       | fennel            | clojure JVM    |
| ------------- | ----------------- | ------------- | ------------- | ------------- | ----------------- | -------------- |
| startup (nil) | 12.8MB (1.0x)     | 26.7MB (2.1x) | 21.2MB (1.7x) | 31.1MB (2.4x) | **3.1MB** (0.2x)  | 92.8MB (7.2x)  |
| fib(35)       | 13.7MB (1.0x)     | 77.1MB (5.6x) | 33.2MB (2.4x) | —MB           | **13.0MB** (0.9x) | 112.1MB (8.2x) |
| reduce 1M     | **19.7MB** (1.0x) | 58.9MB (3.0x) | 33.0MB (1.7x) | 35.3MB (1.8x) | 1122.6MB (57.0x)  | 117.8MB (6.0x) |

### Performance

| Benchmark      | let-go                    | babashka                  | joker                   | glojure                 | fennel                   | clojure JVM                |
| -------------- | ------------------------- | ------------------------- | ----------------------- | ----------------------- | ------------------------ | -------------------------- |
| fib            | 2.059s ± 0.093s (1.0x)    | 2.019s ± 0.078s (1.0x)    | 20.084s ± 0.260s (9.8x) | —                       | 1.955s ± 0.040s (0.9x)   | **0.550s ± 0.015s** (0.3x) |
| loop-recur     | **57.6ms ± 1.0ms** (1.0x) | 65.1ms ± 2.5ms (1.1x)     | 0.700s ± 0.014s (12.2x) | 3.802s ± 0.101s (66.0x) | 0.172s ± 0.003s (3.0x)   | 0.460s ± 0.009s (8.0x)     |
| map-filter     | **7.3ms ± 0.7ms** (1.0x)  | 21.0ms ± 2.0ms (2.9x)     | 13.3ms ± 1.5ms (1.8x)   | 0.161s ± 0.004s (21.9x) | 1.035s ± 0.020s (141.0x) | 0.351s ± 0.006s (47.8x)    |
| persistent-map | **19.0ms ± 1.2ms** (1.0x) | 22.3ms ± 1.2ms (1.2x)     | 48.6ms ± 1.4ms (2.6x)   | 77.6ms ± 3.1ms (4.1x)   | 3.671s ± 0.095s (193.7x) | 0.474s ± 0.007s (25.0x)    |
| reduce         | 78.0ms ± 2.7ms (1.0x)     | **36.6ms ± 2.2ms** (0.5x) | 2.482s ± 0.050s (31.8x) | 1.138s ± 0.140s (14.6x) | 8.099s ± 0.334s (103.9x) | 0.359s ± 0.020s (4.6x)     |
| tak            | 2.045s ± 0.029s (1.0x)    | 1.948s ± 0.065s (1.0x)    | —                       | —                       | 10.716s ± 0.283s (5.2x)  | **0.567s ± 0.038s** (0.3x) |
| transducers    | **6.7ms ± 0.6ms** (1.0x)  | 21.5ms ± 2.4ms (3.2x)     | —                       | 44.7ms ± 0.7ms (6.6x)   | 1.039s ± 0.041s (153.9x) | 0.347s ± 0.009s (51.4x)    |
