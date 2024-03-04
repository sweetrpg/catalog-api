# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""Resource definitions for volumes.
"""

from flask_rest_jsonapi import ResourceList, ResourceDetail, ResourceRelationship
from sweetrpg_library_objects.api.volume.schema import VolumeAPISchema
from sweetrpg_api_core.data import APIData
from sweetrpg_library_objects.model.volume import Volume
from sweetrpg_catalog_api.application.db import db
from sweetrpg_catalog_api.application.blueprints.setup import model_info


class VolumeList(ResourceList):
    """
    Get a list of volumes
    ---
    tags:
        - volumes
    definitions:
        - schema:
            id: Volume
            properties:
            name:
                type: string
                description: the group's name
    parameters:

    responses:
        200:
          description: List of volumes returned
    """
    disable_oauth = True
    schema = VolumeAPISchema
    data_layer = {"class": APIData, "type": "volume", "model": Volume, "db": db, "model_info": model_info}


class VolumeDetail(ResourceDetail):
    disable_oauth = True
    schema = VolumeAPISchema
    data_layer = {
        "class": APIData,
        "type": "volume",
        "model": Volume,
        "db": db,
        "model_info": model_info
    }


class VolumeAuthorRelationship(ResourceRelationship):
    schema = VolumeAPISchema
    data_layer = {
        "class": APIData,
        "type": "volume",
        "model": Volume,
        "db": db,
        "model_info": model_info
    }


class VolumeLicenseRelationship(ResourceRelationship):
    schema = VolumeAPISchema
    data_layer = {
        "class": APIData,
        "type": "volume",
        "model": Volume,
        "db": db,
        "model_info": model_info
    }


class VolumeStudioRelationship(ResourceRelationship):
    schema = VolumeAPISchema
    data_layer = {
        "class": APIData,
        "type": "volume",
        "model": Volume,
        "db": db,
        "model_info": model_info
    }


class VolumePublisherRelationship(ResourceRelationship):
    schema = VolumeAPISchema
    data_layer = {
        "class": APIData,
        "type": "volume",
        "model": Volume,
        "db": db,
        "model_info": model_info
    }


class VolumeSystemRelationship(ResourceRelationship):
    schema = VolumeAPISchema
    data_layer = {
        "class": APIData,
        "type": "volume",
        "model": Volume,
        "db": db,
        "model_info": model_info
    }
