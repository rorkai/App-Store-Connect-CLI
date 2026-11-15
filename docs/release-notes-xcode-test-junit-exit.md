# `asc xcode test junit` exits 1 on failed tests

Changed in 7.0.0. `asc xcode test junit` now exits `1` when the result bundle
contains at least one failed test, the same exit code as a failed
`asc xcode test` run. Earlier releases exited `0` whatever the bundle
contained, so a CI step that only converted results passed on a failed run.

The JUnit report and the receipt on stdout are still written in full before
the command exits. Stderr then reports the failure count:

```text
Error: xcode test junit: 1 failed test(s); report written to ./junit.xml
```

## Migration

To keep the old behaviour, for example when a later step decides pass or fail
from the report, add `--allow-failures`:

```bash
asc xcode test junit --xcresult ./Test.xcresult --report-file ./junit.xml --allow-failures --output json
```
