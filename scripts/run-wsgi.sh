#!/bin/bash

set -x
set -e
set -o pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

export FLASK_APP=sweetrpg_library_api.application.main:create_app
export FLASK_ENV=production

pushd src

export $(cat ${scriptdir}/../src/configs/local/wsgi.env | xargs)

gunicorn --log-level debug -w 1 wsgi:app

popd
