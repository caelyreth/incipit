# Word Segmenter Provenance

The segmenter's code is a CJK/Latin extraction of PiG's
`internal/wordsegmenter`:

```text
https://github.com/MichaelKinsy/PiG/tree/main/internal/wordsegmenter
upstream commit c475b56d028166a429b7fe35476520550780a471
```

The extraction keeps Unicode 17 UAX #29 scanning, ICU 78.3 word-rule
tailoring, the ICU Chinese/Japanese dictionary engine and generated rule
data. It removes the Thai, Lao, Khmer, and Burmese engines and their
dictionaries, plus PiG's application-specific strip state and WTF-8
adapter.

The dictionary is no longer PiG's: `scripts/cjkdict` builds it from ICU's
own word list, in Neverland's format (described in that command), and
`scripts/fetch-assets.sh` runs it. The index's words and costs are those of
PiG's former `cjk_dictionary.bin`, byte for byte, after the format's
header.

Pinned data:

```text
ICU release-78.3 cjdict.txt sha256:
e73fd72048981d0cc13e9dc436a7eaba07ffb6eff58c8a59dc75c1df746663a0

cjk_dictionary.bin (Neverland format, version 1) sha256:
35ca601ac1c6fbc9900ee1d692c32c2a958254f6a9f31c77e841ee09863db577
```

PiG is MIT licensed. The translated ICU engine and the CJK dictionary built
from ICU's data retain the notices under `LICENSES/`.

The dictionary is not stored in Git.
