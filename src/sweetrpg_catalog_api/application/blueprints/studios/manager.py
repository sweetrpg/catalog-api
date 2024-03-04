# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

from flask_rest_jsonapi import ResourceList, ResourceDetail, ResourceRelationship
from  sweetrpg_library_objects.api.studio.schema import StudioAPISchema
from sweetrpg_api_core.data import APIData
from sweetrpg_library_objects.model.studio import Studio
from sweetrpg_catalog_api.application.db import db
from sweetrpg_catalog_api.application.blueprints.setup import model_info


class StudioList(ResourceList):
    schema = StudioAPISchema
    data_layer = {"class": APIData, "type": "studio", "model": Studio, "db": db, "model_info": model_info}


class StudioDetail(ResourceDetail):
    schema = StudioAPISchema
    data_layer = {
        "class": APIData,
        "type": "studio",
        "model": Studio,
        "db": db,
        "model_info": model_info
    }


# class StudioAuthorRelationship(ResourceRelationship):
#     schema = StudioAPISchema
#     data_layer = {
#         "class": APIData,
#         "type": "studio",
#         "model": Studio,
#         "db": db,
#         "model_info": model_info
#     }
