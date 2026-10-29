# Performance

Run the repeatable benchmark suite with:

```sh
./scripts/bench-performance.sh
```

The script covers long Latin/CJK prose, ordinary and heavy-policy typing,
retained native paragraph layout, caret/selection geometry, headless paint,
and end-to-end controller-to-paint typing. It writes raw results under the
Git-ignored `references/` archive.

## Reference Result

Measured on an Apple M5 Pro with Go 1.27.3, `-benchtime=1s -count=3`.

One keystroke plus a complete native rebuild and paint:

| Initial text | Before | After | Improvement |
| --- | ---: | ---: | ---: |
| 64 KiB | 10.15 ms, 5.30 MB | 1.27 ms, 359 KB | 8.0x faster |
| 256 KiB | 87.8 ms, 21.8 MB | 3.77 ms, 1.03 MB | 23.3x faster |

One changed paragraph in retained native layout:

| Initial text | Before | After | Improvement |
| --- | ---: | ---: | ---: |
| 64 KiB | 3.06 ms | 0.147 ms | 20.8x faster |
| 256 KiB | 44-47 ms | 0.579 ms | about 78x faster |

The complete evidence, profiles, implementation ledger, and remaining risks
are under `references/performance-2026-10-29/`. These are development
measurements, not universal release latency guarantees.
