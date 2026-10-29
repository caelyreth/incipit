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

PiG is MIT licensed. The translated ICU engine and embedded CJK dictionary
retain the notices under `LICENSES/`.
