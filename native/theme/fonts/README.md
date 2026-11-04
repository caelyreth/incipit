# Bundled Fonts

Neither font is stored in Git. Fetch both with:

```sh
./scripts/fetch-fonts.sh
```

The same command is wired as `go generate ./native/theme`. It downloads each
font to its ignored path here, verifies its checksum, and installs it
atomically.

## Prose

`NotoSerifSC-Variable.ttf` is the Noto Serif SC variable font from Google
Fonts:

```text
https://github.com/google/fonts/tree/main/ofl/notoserifsc
https://raw.githubusercontent.com/google/fonts/main/ofl/notoserifsc/NotoSerifSC%5Bwght%5D.ttf
SHA-256 050080d9255a86808f2945bffac582b31ef32bc36411ce29563b4961670c66f9
```

It is registered under Neverland's source family name,
`Noto Serif CJK SC`, so document typography does not depend on fonts
installed by the host operating system. It is licensed under the SIL Open
Font License 1.1 in `OFL.txt`.

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
