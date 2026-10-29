# Word Segmenter Provenance

This package is a CJK/Latin extraction of PiG's `internal/wordsegmenter`:

```text
https://github.com/MichaelKinsy/PiG/tree/main/internal/wordsegmenter
upstream commit c475b56d028166a429b7fe35476520550780a471
```

The extraction keeps Unicode 17 UAX #29 scanning, ICU 78.3 word-rule
tailoring, the ICU Chinese/Japanese dictionary engine, generated rule data, and
the `cjdict.txt` index. It removes the Thai, Lao, Khmer, and Burmese engines and
their dictionaries, plus PiG's application-specific strip state and WTF-8
adapter.

Pinned data:

```text
ICU cjdict.txt source sha256:
e73fd72048981d0cc13e9dc436a7eaba07ffb6eff58c8a59dc75c1df746663a0

embedded cjk_dictionary.bin sha256:
5111be4597ef8370329df83b3a3f1f59628ee7c4eb3b30208df02f56062f0010
```

PiG is MIT licensed. The translated ICU engine and embedded CJK dictionary
retain the notices under `LICENSES/`.
