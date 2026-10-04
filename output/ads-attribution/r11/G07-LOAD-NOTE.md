# Final G07 workload provenance

The final-source run started after three quiet samples at01:38UTC:
load1=6.08/5.73/4.34 and no other `foundation.test` process.

At02:01:57UTC the sampler observed load1=18.50 (previously mostly2–5),
then14.19 at02:02. Read-only process inspection found a concurrent Humaux Rust
`contribution_repo` test in addition to the virtual-machine and system processes.
This observation does not assign the whole spike to one process. No foreign
process was stopped or modified. The full run was therefore not continuously
idle; do not describe it that way. `load-spike-0202.txt` and `machine-load.log`
preserve the observations. Final functional result is **PASS, exit0**:
2007 top-level passes,0 failures;6687 passes including subtests. The30-second
sampler contains119 observations from the run, min2.03/max16.31. Its cadence
differs from the live monitor that observed18.50; both are retained. Final
source postflight confirms068874fc and zero runtime/test source differences.

The transient Rust process had ended before the later targeted `lsof` lookup,
so that read-only lookup returned1; it was not a test failure and was not used
to assert a working-directory attribution. No whole-spike causal attribution
or continuously-idle claim is made.
