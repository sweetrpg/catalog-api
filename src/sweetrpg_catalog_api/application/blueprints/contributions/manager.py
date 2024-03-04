# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

from flask import current_app
from flask_rest_jsonapi import ResourceList, ResourceDetail, ResourceRelationship
from sweetrpg_api_core.data import APIData
from sweetrpg_library_objects.api.contribution.schema import ContributionAPISchema
from sweetrpg_library_objects.model.contribution import Contribution

from sweetrpg_catalog_api.application.auth import oauth
from sweetrpg_catalog_api.application.blueprints.setup import model_info
from sweetrpg_catalog_api.application.db import db


class ContributionList(ResourceList):
    disable_oauth = True
    schema = ContributionAPISchema
    data_layer = {"class": APIData, "type": "contribution", "model": Contribution, "db": db, "model_info": model_info}

    @oauth.require_oauth('create_contribution')
    def post(*args, **kwargs):
        current_app.logger.debug("args: %s, kwargs: %s", args, kwargs)
        return "TODO"


class ContributionDetail(ResourceDetail):
    disable_oauth = True
    schema = ContributionAPISchema
    data_layer = {"class": APIData, "type": "contribution", "model": Contribution, "db": db, "model_info": model_info}

    @oauth.require_oauth('update_contribution')
    def patch(*args, **kwargs):
        current_app.logger.debug("args: %s, kwargs: %s", args, kwargs)
        return "TODO"

    @oauth.require_oauth('delete_contribution')
    def delete(*args, **kwargs):
        current_app.logger.debug("args: %s, kwargs: %s", args, kwargs)
        return "TODO"


class ContributionVolumeRelationship(ResourceRelationship):
    disable_oauth = True
    schema = ContributionAPISchema
    data_layer = {"class": APIData, "type": "contribution", "model": Contribution, "db": db, "model_info": model_info}


class ContributionPersonRelationship(ResourceRelationship):
    disable_oauth = True
    schema = ContributionAPISchema
    data_layer = {"class": APIData, "type": "contribution", "model": Contribution, "db": db, "model_info": model_info}
