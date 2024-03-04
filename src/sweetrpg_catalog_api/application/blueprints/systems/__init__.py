# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

# from flask_rest_jsonapi import Api
from .manager import SystemList, SystemDetail
from ...blueprints import api


def setup_routes(app):
    # app.logger.info("Registering system models...")
    # for model_name, model_info in system_model_info.items():
    #     APIData.add_model(model_name, model_info)

    app.logger.info("Setting up routes for Systems API")
    # api = Api(app)
    api.route(SystemList, "system_list", "/systems/")
    api.route(SystemDetail, "system_detail", "/systems/<id>")
    # api.route(SystemAuthorRelationship, "system_authors", "/systems/<id>/relationships/persons")
