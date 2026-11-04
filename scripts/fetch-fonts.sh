#!/bin/sh
set -eu

root=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
fonts="$root/native/theme/fonts"

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

# fetch name file url sha256 installs one font at fonts/file, verified.
fetch() {
	name=$1
	destination="$fonts/$2"
	url=$3
	expected=$4
	if [ -f "$destination" ]; then
		actual=$(sha256_file "$destination")
		if [ "$actual" = "$expected" ]; then
			echo "$name is already present and verified."
			return 0
		fi
		echo "Replacing mismatched $name font." >&2
	fi

	mkdir -p "$fonts"
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

fetch 'Noto Serif SC' NotoSerifSC-Variable.ttf \
	'https://raw.githubusercontent.com/google/fonts/main/ofl/notoserifsc/NotoSerifSC%5Bwght%5D.ttf' \
	'050080d9255a86808f2945bffac582b31ef32bc36411ce29563b4961670c66f9'
fetch 'New Computer Modern Math' NewCMMath-Book.otf \
	'https://raw.githubusercontent.com/typst/typst-assets/94dcb990edb36311ef85162baa26e4a8205e0bf0/files/fonts/NewCMMath-Book.otf' \
	'2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95'
