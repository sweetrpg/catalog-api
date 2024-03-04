# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
__init__.py
Application root module.
"""

import os

import sentry_sdk
from sentry_sdk.integrations.flask import FlaskIntegration
from sentry_sdk.integrations.redis import RedisIntegration

from sweetrpg_catalog_api.application import constants

sentry_sdk.init(dsn=os.environ[constants.SENTRY_DSN],
                traces_sample_rate=0.2,
                environment=os.environ.get(constants.SENTRY_ENV) or 'Unknown',
                integrations=[
                    FlaskIntegration(), RedisIntegration(),
                ])
