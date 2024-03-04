#!/bin/bash

set -e

#newrelic-admin run-program gunicorn wsgi:app
uwsgi \
    --logformat '%(addr) - %(user) [%(ltime)] "%(method) %(uri) %(proto)" %(status) %(size) "%(referer)" "%(uagent)"' \
    --http :${PORT} \
    --module wsgi:app \
    --master \
    --processes 4 \
    --threads 2
