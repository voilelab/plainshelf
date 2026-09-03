## 問題

<!-- The symptom, and the cause traced in the repository's own terms. -->

## 改法

<!-- What changed, and why at this layer rather than another. -->

## Test levels

<!--
Which levels this change is tested at, and why not the others.
See docs/development/testing-levels.md — R1 says lowest level first, and R2
says "no automated test" is a legal answer as long as it is stated.
An added E2E case is net-zero (R4): name the case deleted or merged for it.
-->

- [ ] L1 Go unit
- [ ] L2 Go API contract
- [ ] L3 Frontend unit / component
- [ ] L4 E2E — net-zero, removed/merged:
- [ ] L5 Manual only — see below
- [ ] No automated test (R2), because:

## Manual verification

<!--
Required for L5 behavior, and for anything CI cannot reach.
Leave "n/a" when the change is fully covered by L1-L4.
-->

- **環境**: <!-- OS / device / browser / server build -->
- **步驟**: <!-- what you did, in order -->
- **結果**: <!-- what you observed, including what did not happen -->
- **截圖**: <!-- attach, or state why one does not apply -->

## 檢查

<!-- Commands run, and explicitly the gates that were not run. -->

```
```
