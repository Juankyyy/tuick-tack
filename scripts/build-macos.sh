#!/usr/bin/env bash
# Build or package Tuick-Tack for macOS. Run it from anywhere; paths are
# resolved from the repository root. The build mode needs Xcode Command Line
# Tools for cgo, and the package mode needs lipo, iconutil, codesign and ditto
# (all part of macOS).
#
#   scripts/build-macos.sh build -version <vX.Y.Z|dev> -arch <arm64|amd64>
#   scripts/build-macos.sh package -version <vX.Y.Z|dev> \
#       -arm64 <binary> -amd64 <binary>
set -euo pipefail

usage() {
	cat <<'EOF'
usage:
  scripts/build-macos.sh build -version <vX.Y.Z|dev> -arch <arm64|amd64>
  scripts/build-macos.sh package -version <vX.Y.Z|dev> -arm64 <binary> -amd64 <binary>

build:   compiles dist/tuick-tack-darwin-<arch> with cgo.
package: combines the two binaries with lipo, builds Tuick-Tack.app from the
         committed Info.plist, wrapper and icon.iconset, signs it ad-hoc and
         writes dist/tuick-tack-macos-universal.zip plus SHA256SUMS.txt.
EOF
}

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

if [ $# -eq 0 ]; then
	usage >&2
	exit 2
fi
mode=$1
shift

version=dev
arch=
arm64=
amd64=

while [ $# -gt 0 ]; do
	case "$1" in
	-version)
		[ $# -ge 2 ] || { usage >&2; exit 2; }
		version=$2
		shift 2
		;;
	-arch | -arm64 | -amd64)
		[ $# -ge 2 ] || { usage >&2; exit 2; }
		case "$1" in
		-arch) arch=$2 ;;
		-arm64) arm64=$2 ;;
		-amd64) amd64=$2 ;;
		esac
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "build-macos.sh: unknown argument: $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

case "$mode" in
build | package) ;;
*)
	echo "build-macos.sh: unknown mode: $mode" >&2
	usage >&2
	exit 2
	;;
esac

# CFBundleShortVersionString wants one to three numeric components. Development
# builds use 0.0.0 in the bundle while internal/buildinfo.Version keeps "dev";
# a prerelease or malformed version is rejected instead of silently stamped.
bundle_version() {
	local given=$1
	if [ "$given" = "dev" ]; then
		echo 0.0.0
		return 0
	fi
	local raw=${given#v}
	raw=${raw#V}
	local IFS=.
	# The split must not glob: a version is data, not a pattern.
	set -f
	# shellcheck disable=SC2086
	set -- $raw
	set +f
	if [ $# -lt 1 ] || [ $# -gt 3 ]; then
		echo "build-macos.sh: version must be vX.Y.Z or dev (got: $given)" >&2
		return 1
	fi
	for part in "$@"; do
		case "$part" in
		'' | *[!0-9]*)
			echo "build-macos.sh: version must be stable and numeric (got: $given)" >&2
			return 1
			;;
		esac
	done
	echo "$raw"
}

mkdir -p dist

case "$mode" in
build)
	[ -n "$arch" ] || { echo "build-macos.sh: build needs -arch" >&2; exit 2; }
	case "$arch" in
	arm64 | amd64) ;;
	*)
		echo "build-macos.sh: unsupported arch: $arch" >&2
		exit 2
		;;
	esac
	echo "==> building $version for darwin/$arch"
	CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" \
		go build -trimpath \
		-ldflags "-s -w -X tuick-tack/internal/buildinfo.Version=$version" \
		-o "dist/tuick-tack-darwin-$arch" ./cmd/tuick-tack
	;;
package)
	[ -n "$arm64" ] && [ -f "$arm64" ] || {
		echo "build-macos.sh: package needs -arm64 <binary>" >&2
		exit 2
	}
	[ -n "$amd64" ] && [ -f "$amd64" ] || {
		echo "build-macos.sh: package needs -amd64 <binary>" >&2
		exit 2
	}
	bv=$(bundle_version "$version") || exit 2
	app=dist/Tuick-Tack.app
	stage=dist/macos-portable
	rm -rf "$app" "$stage" dist/tuick-tack-macos-universal.zip dist/SHA256SUMS.txt
	mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"

	echo "==> combining arm64 and amd64"
	lipo -create -output "$app/Contents/MacOS/tuick-tack" "$arm64" "$amd64"
	cp macos/launcher "$app/Contents/MacOS/launcher"
	chmod 755 "$app/Contents/MacOS/launcher" "$app/Contents/MacOS/tuick-tack"

	echo "==> writing Info.plist ($bv)"
	sed "s/__VERSION__/$bv/g" macos/Info.plist >"$app/Contents/Info.plist"
	plutil -lint "$app/Contents/Info.plist" >/dev/null

	echo "==> building icon.icns"
	iconutil -c icns assets/icon.iconset -o "$app/Contents/Resources/icon.icns"

	echo "==> signing ad-hoc"
	# Stray extended attributes would travel into the zip as AppleDouble files and
	# make codesign report a modified bundle after unzipping.
	xattr -cr "$app" 2>/dev/null || true
	codesign --force --sign - "$app/Contents/MacOS/tuick-tack"
	codesign --force --sign - "$app"
	codesign --verify --deep --strict --verbose=2 "$app"

	# The zip root holds the app and the Gatekeeper notes, nothing else.
	mkdir -p "$stage"
	cp -R "$app" "$stage/"
	cp macos/README.txt "$stage/README.txt"
	echo "==> writing tuick-tack-macos-universal.zip"
	ditto -c -k --norsrc --noextattr "$stage" dist/tuick-tack-macos-universal.zip
	(
		cd dist
		shasum -a 256 tuick-tack-macos-universal.zip >SHA256SUMS.txt
	)

	"$app/Contents/MacOS/tuick-tack" --version
	echo "==> dist/tuick-tack-macos-universal.zip"
	;;
esac
