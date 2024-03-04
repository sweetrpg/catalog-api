# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

# from flask_rest_jsonapi import Api
from .manager import StudioList, StudioDetail
from ...blueprints import api


def setup_routes(app):
    # app.logger.info("Registering studio models...")
    # for model_name, model_info in studio_model_info.items():
    #     APIData.add_model(model_name, model_info)

    app.logger.info("Setting up routes for Studios API")
    # api = Api(app)
    api.route(StudioList, "studio_list", "/studios/")
    api.route(StudioDetail, "studio_detail", "/studios/<id>")
    # api.route(StudioAuthorRelationship, "studio_authors", "/studios/<id>/relationships/persons")
