#!/bin/sh
set -eu

root=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
destination="$root/native/theme/fonts/NotoSerifSC-Variable.ttf"
url='https://raw.githubusercontent.com/google/fonts/main/ofl/notoserifsc/NotoSerifSC%5Bwght%5D.ttf'
expected='050080d9255a86808f2945bffac582b31ef32bc36411ce29563b4961670c66f9'

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

if [ -f "$destination" ]; then
	actual=$(sha256_file "$destination")
	if [ "$actual" = "$expected" ]; then
		echo "Noto Serif SC is already present and verified."
		exit 0
	fi
	echo "Replacing mismatched Noto Serif SC font." >&2
fi

mkdir -p "$(dirname "$destination")"
temporary=$(mktemp "$destination.tmp.XXXXXX")
cleanup() {
	rm -f "$temporary"
}
trap cleanup EXIT HUP INT TERM

echo "Fetching Noto Serif SC..."
curl --fail --location --silent --show-error --retry 4 --retry-delay 1 \
	--retry-all-errors \
	"$url" --output "$temporary"

actual=$(sha256_file "$temporary")
if [ "$actual" != "$expected" ]; then
	echo "error: Noto Serif SC checksum mismatch" >&2
	echo "expected: $expected" >&2
	echo "actual:   $actual" >&2
	exit 1
fi

chmod 0644 "$temporary"
mv "$temporary" "$destination"
trap - EXIT HUP INT TERM
echo "Installed Noto Serif SC at $destination"
