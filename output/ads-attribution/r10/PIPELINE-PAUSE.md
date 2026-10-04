# Source 7933d449 verification, not the final acceptance source

Static gates, 359 Node tests, 39 focused PG tests, the attribution browser gate,
and the complete click sweep finished with exit 0 (see `exit-codes.tsv`).

Independent visual inspection then confirmed overlapping English desktop table
headers. Existing numeric and page-width checks did not detect this local text
collision. The layout regression is being added before a narrowly scoped CSS fix.

After the click-sweep harness returned and completed cleanup, the root stopped
its own pipeline PID 75918 during the quiet-machine preflight. Pipeline exit 143
is an intentional orchestration stop, not a test failure. G07 did NOT start on
7933d449. The final G07 must compile the later frozen source that includes the
layout correction. All r10 results and images remain historical evidence.
