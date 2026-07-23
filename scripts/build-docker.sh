#!/bin/bash

set -x
set -e
set -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

cd $scriptdir/..

docker build -t ghcr.io/sweetrpg/catalog-api . $@
