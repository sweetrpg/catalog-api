# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

import logging

from sweetrpg_api_core.blueprints.health import register_health_check_service_hook

from sweetrpg_catalog_api.application.cache import cache
from sweetrpg_catalog_api.application.db import db


def _db_check():
    database = db.connection.get_default_database()
    return {
        # 'db': str(db),
        "connection": db.connection.server_info(),
        "address": str(db.connection.address),
        "name": database.name,
        "collections": database.list_collection_names(),
    }


def _cache_check():
    return {
        **cache.config,
    }


def register_service_checks():
    logging.info("Registering health check service hook for 'database'...")
    register_health_check_service_hook("database", _db_check)
    register_health_check_service_hook("cache", _cache_check)
