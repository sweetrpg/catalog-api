#!/bin/bash

set -x
set -e
set -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

export FLASK_APP=sweetrpg_library_api.application.main:create_app
export FLASK_ENV=development

pushd src

export $(cat ${scriptdir}/../src/configs/local/local.env | xargs)

python3 appserver.py

popd
