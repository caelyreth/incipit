#!/bin/sh
set -eu

# fetch-assets.sh installs the binary assets Git does not store: the
# fonts, each from a pinned URL, and the word segmenter's CJK dictionary,
# built from ICU's pinned word list; each is verified against its SHA-256
# before it is installed.

root=$(CDPATH= cd "$(dirname "$0")/.." && pwd)

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		echo "error: sha256sum or shasum is required" >&2
		return 1
	fi
}

# fetch name path url sha256 installs one asset at path (from the
# repository root), verified.
fetch() {
	name=$1
	destination="$root/$2"
	url=$3
	expected=$4
	if [ -f "$destination" ]; then
		actual=$(sha256_file "$destination")
		if [ "$actual" = "$expected" ]; then
			echo "$name is already present and verified."
			return 0
		fi
		echo "Replacing mismatched $name." >&2
	fi

	mkdir -p "$(dirname "$destination")"
	temporary=$(mktemp "$destination.tmp.XXXXXX")
	trap 'rm -f "$temporary"' EXIT HUP INT TERM

	echo "Fetching $name..."
	curl --fail --location --silent --show-error --retry 4 --retry-delay 1 \
		--retry-all-errors \
		"$url" --output "$temporary"

	actual=$(sha256_file "$temporary")
	if [ "$actual" != "$expected" ]; then
		echo "error: $name checksum mismatch" >&2
		echo "expected: $expected" >&2
		echo "actual:   $actual" >&2
		exit 1
	fi

	chmod 0644 "$temporary"
	mv "$temporary" "$destination"
	trap - EXIT HUP INT TERM
	echo "Installed $name at $destination"
}

# Noto Serif, the prose face, Latin, Greek and Cyrillic: other scripts'
# faces are downloaded in the app, when asked (app/fonts).
fetch 'Noto Serif' native/theme/fonts/NotoSerif-Variable.ttf \
	'https://raw.githubusercontent.com/google/fonts/06bfe0d62a98375cb09c942fa18e68bd32cff057/ofl/notoserif/NotoSerif%5Bwdth%2Cwght%5D.ttf' \
	'4d8e6761424656867019081a1a01336f3cb086982682698714054fc33f782713'
fetch 'Noto Serif Italic' native/theme/fonts/NotoSerif-Italic-Variable.ttf \
	'https://raw.githubusercontent.com/google/fonts/06bfe0d62a98375cb09c942fa18e68bd32cff057/ofl/notoserif/NotoSerif-Italic%5Bwdth%2Cwght%5D.ttf' \
	'e87acbc6c0efd0d9a20d6a8cbbda2b266c14be3a3a6f5af8ec9d7b2460570ad1'
fetch 'New Computer Modern Math' native/theme/fonts/NewCMMath-Book.otf \
	'https://raw.githubusercontent.com/typst/typst-assets/94dcb990edb36311ef85162baa26e4a8205e0bf0/files/fonts/NewCMMath-Book.otf' \
	'2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95'
# ICU release-78.3 cjdict.txt, indexed in Neverland's format for the word
# segmenter, both pinned (scripts/cjkdict).
# go generate names the toolchain it runs in GOROOT.
(cd "$root" && "${GOROOT:+$GOROOT/bin/}go" run ./scripts/cjkdict)
