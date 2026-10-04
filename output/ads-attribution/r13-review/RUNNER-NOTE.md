# Historical runner exit note

The preliminary `0cab39ed` focused command itself exited **1**, with 88 top-level
PASS, 4 FAIL, 4 prerequisite SKIP; this is recorded by test-focused and results.tsv.
The surrounding evidence shell subsequently exited **2** because the author
added optional edge/G04 dispatch branches to its on-disk script while that shell
was still executing the long command. Its later file read encountered a changed
offset and reported an unmatched quote. The final on-disk script passes `bash -n`.

This does not turn a failed gate green. The whole run is retained as RED in
`focused-0cab39ed-red.log`. Do not edit the evidence runner during another run;
freeze its bytes along with final source and wait for completion before changes.
No G07 was active during this incident.
