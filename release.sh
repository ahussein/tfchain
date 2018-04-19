#!/bin/bash
set -e

package="github.com/threefoldfoundation/tfchain"

version="$(git describe | cut -d '-' -f 1)"
commit="$(git rev-parse --short HEAD)"
if [ "$commit" == "$(git rev-list -n 1 $version | cut -c1-7)" ]
then
	full_version="$version"
else
	full_version="${version}-${commit}"
fi

for os in darwin linux windows; do
	echo Packaging ${os}...
	# create workspace
	folder="release/tfchain-${version}-${os}-amd64"
	rm -rf "$folder"
	mkdir -p "$folder"
	# compile and sign binaries
	for pkg in cmd/tfchainc cmd/tfchaind; do
		bin=$pkg
		if [ "$os" == "windows" ]; then
			bin=${pkg}.exe
		fi
		GOOS=${os} go build -a \
			-ldflags="-X ${package}/pkg/config.rawVersion=${full_version} -s -w" \
			-o "${folder}/${bin}" "./${pkg}"

	done
	# add other artifacts
	cp -r LICENSE README.md "$folder"
	# zip
	(
		zip -rq "release/tfchain-${version}-${os}-amd64.zip" \
			"release/tfchain-${version}-${os}-amd64"
	)
done