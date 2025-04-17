#!/bin/bash

set -x
set -e
set -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

cd $scriptdir/../docker/
docker-compose -f docker-compose-atlas.yml up --force-recreate
