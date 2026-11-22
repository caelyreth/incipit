# Bundled Fonts

No font is stored in Git. Fetch them with:

```sh
./scripts/fetch-assets.sh
```

The same command is wired as `go generate ./native/theme`. It downloads each
font to its ignored path here, verifies its checksum, and installs it
atomically.

## Prose

`NotoSerif-Variable.ttf` and `NotoSerif-Italic-Variable.ttf` are Noto Serif,
upright and italic, variable in width and weight, from Google Fonts, at a
pinned commit of its repository:

```text
https://github.com/google/fonts/tree/06bfe0d62a98375cb09c942fa18e68bd32cff057/ofl/notoserif
NotoSerif[wdth,wght].ttf         SHA-256 4d8e6761424656867019081a1a01336f3cb086982682698714054fc33f782713
NotoSerif-Italic[wdth,wght].ttf  SHA-256 e87acbc6c0efd0d9a20d6a8cbbda2b266c14be3a3a6f5af8ec9d7b2460570ad1
```

Both are registered under Neverland's family name, `Noto Serif`, so
document typography does not depend on fonts installed by the host
operating system. They draw Latin, Greek and Cyrillic. The app ships no face
for Chinese and Japanese: the settings' Writing Systems section offers
Noto Serif SC, TC, HK and JP, and downloads the one the person chooses,
Google Fonts' static builds at weights 400 and 700 from `fonts.gstatic.com`,
or its mirror `fonts.gstatic.cn` when that fails, verified by the SHA-256
pinned in `app/fonts/catalog.go`, and keeps it on that machine. The catalog
is written by `go run ./scripts/fontcatalog`, which asks Google Fonts' CSS
API for the current files and pins each. With none chosen, Chinese and
Japanese, and every other script, are drawn in the system's fonts. The Noto Serif faces
are licensed under the SIL Open Font License 1.1 in `OFL.txt`.

## Math

`NewCMMath-Book.otf` is New Computer Modern Math, Book weight, as pinned by
Typst's assets and by the untie renderer's own test fixtures:

```text
https://github.com/typst/typst-assets/blob/94dcb990edb36311ef85162baa26e4a8205e0bf0/files/fonts/NewCMMath-Book.otf
SHA-256 2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95
```

Formulas are laid out from its OpenType MATH table by untie, which reads the
font's bytes directly; it is not registered with the platform. New Computer
Modern is licensed under the GUST Font License
(https://www.gust.org.pl/projects/e-foundry/licenses).
