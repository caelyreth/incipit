# Bundled Prose Font

`NotoSerifSC-Variable.ttf` is the Noto Serif SC variable font from Google
Fonts:

```text
https://github.com/google/fonts/tree/main/ofl/notoserifsc
https://raw.githubusercontent.com/google/fonts/main/ofl/notoserifsc/NotoSerifSC%5Bwght%5D.ttf
SHA-256 050080d9255a86808f2945bffac582b31ef32bc36411ce29563b4961670c66f9
```

The font is not stored in Git. Fetch it with:

```sh
./scripts/fetch-fonts.sh
```

The same command is wired as `go generate ./native/theme`. It downloads to
`native/theme/fonts/NotoSerifSC-Variable.ttf`, verifies the checksum, and
atomically installs the file at the ignored local path.

It is registered under Shoreward's source family name,
`Noto Serif CJK SC`, so document typography does not depend on fonts
installed by the host operating system.

The font is licensed under the SIL Open Font License 1.1 in `OFL.txt`.
