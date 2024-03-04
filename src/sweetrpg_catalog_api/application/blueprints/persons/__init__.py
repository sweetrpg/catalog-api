# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

from .manager import PersonList, PersonDetail, PersonVolumeRelationship
from ...blueprints import api


def setup_routes(app):
    # app.logger.info("Registering person models...")
    # for model_name, model_info in person_model_info.items():
    #     APIData.add_model(model_name, model_info)

    app.logger.info("Setting up routes for Persons API")
    # api = Api(app)
    api.route(PersonList, "person_list", "/persons/")
    api.route(PersonDetail, "person_detail", "/persons/<id>")
    api.route(PersonVolumeRelationship, "person_volumes", "/persons/<id>/relationships/volumes")
