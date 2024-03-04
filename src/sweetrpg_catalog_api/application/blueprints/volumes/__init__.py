# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""Volume routes.
"""

# from flask_rest_jsonapi import Api
from .manager import VolumeList, VolumeDetail, VolumeAuthorRelationship, VolumeLicenseRelationship, VolumeStudioRelationship, VolumePublisherRelationship, VolumeSystemRelationship
from ...blueprints import api


def setup_routes(app):
    # app.logger.info("Registering volume models...")
    # for model_name, model_info in volume_model_info.items():
    #     APIData.add_model(model_name, model_info)

    app.logger.info("Setting up routes for Volumes API")
    # api = Api(app)
    api.route(VolumeList, "volume_list", "/volumes/")
    api.route(VolumeDetail, "volume_detail", "/volumes/<id>")
    api.route(VolumeAuthorRelationship, "volume_authors", "/volumes/<id>/relationships/persons")
    api.route(VolumeLicenseRelationship, "volume_licenses", "/volumes/<id>/relationships/licenses")
    api.route(VolumeStudioRelationship, "volume_studios", "/volumes/<id>/relationships/studios")
    api.route(VolumePublisherRelationship, "volume_publishers", "/volumes/<id>/relationships/publishers")
    api.route(VolumeSystemRelationship, "volume_systems", "/volumes/<id>/relationships/systems")
