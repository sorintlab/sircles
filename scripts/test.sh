#!/bin/bash

set -e

FMTOUT=$(gofmt -l $(find . -type f -name '*.go' | grep -v vendor | grep -v webbundle/bindata.go))
if [ -n "${FMTOUT}" ]; then
	echo -e "gofmt checking failed:\n${FMTOUT}"
	exit 255
fi

PACKAGES=$(go list ./... | grep -v /vendor/)

# github.com/sorintlab/sircles/api/graphql can be considered an integration test since it call all the underlying services. We want to see the code coverage of all the sircles packages

TMPDIR="$(mktemp -d)"

i=0
for p in $PACKAGES; do
  echo "Testing ${p}"
  COVERPKG=$(go list -f '{{ join .Deps "\n" }}' ${p} | grep github.com/sorintlab/sircles/ | grep -v vendor | tr '\n' ',' )
  # add the tested package to covered packages, COVERPKG already ends with a comma.
  COVERPKG="${COVERPKG}${p}"
  go test -v -coverprofile=${TMPDIR}/coverprofile.${i} -coverpkg "$COVERPKG" $p
  i=$((i+1))
done

# merge all coverprofiles
./tools/bin/gocovmerge ${TMPDIR}/coverprofile.* > ${TMPDIR}/coverprofile

echo "== Total coverage =="
go tool cover -func ${TMPDIR}/coverprofile | tail -1
