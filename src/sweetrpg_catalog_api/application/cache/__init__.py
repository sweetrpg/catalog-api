# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""


from flask_caching import Cache
import os


cache = Cache(config={
    'CACHE_TYPE': 'redis',
    'CACHE_KEY_PREFIX': 'sweetrpg',
    'CACHE_OPTIONS': {},
    # 'CACHE_REDIS_HOST': os.environ['REDIS_HOST'],
    # 'CACHE_REDIS_PORT': os.environ.get('REDIS_PORT') or 6379,
    # 'CACHE_REDIS_PASSWORD': os.environ['REDIS_PASSWORD'],
    # 'CACHE_REDIS_DB': os.environ.get('REDIS_DB') or 86,
    })
