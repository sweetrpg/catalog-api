# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

from flask import current_app
from flask_rest_jsonapi import ResourceList, ResourceDetail, ResourceRelationship
from  sweetrpg_library_objects.api.review.schema import ReviewAPISchema
from sweetrpg_api_core.data import APIData
from sweetrpg_library_objects.model.review import Review
from sweetrpg_catalog_api.application.db import db
from sweetrpg_catalog_api.application.blueprints.setup import model_info
from sweetrpg_catalog_api.application.auth import oauth
import logging


class ReviewList(ResourceList):
    schema = ReviewAPISchema
    data_layer = {"class": APIData, "type": "review", "model": Review, "db": db, "model_info": model_info}


class ReviewDetail(ResourceDetail):
    # disable_oauth = True
    schema = ReviewAPISchema
    data_layer = {
        "class": APIData,
        "type": "review",
        "model": Review,
        "db": db,
        "model_info": model_info
    }
    # decorators = (check_auth,)

    # @oauth.require_oauth("something")
    # def create(*args, **kwargs):
    #     current_app.logger.debug("args: %s, kwargs: %s", args, kwargs)
    #     return "TODO"


# class ReviewAuthorRelationship(ResourceRelationship):
#     schema = ReviewAPISchema
#     data_layer = {
#         "class": APIData,
#         "type": "review",
#         "model": Review,
#         "db": db,
#         "model_info": model_info
#     }
