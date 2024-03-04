# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

# from flask_rest_jsonapi import Api
from .manager import PublisherList, PublisherDetail
from ...blueprints import api


def setup_routes(app):
    # app.logger.info("Registering publisher models...")
    # for model_name, model_info in publisher_model_info.items():
    #     APIData.add_model(model_name, model_info)

    app.logger.info("Setting up routes for Publishers API")
    # api = Api(app)
    api.route(PublisherList, "publisher_list", "/publishers/")
    api.route(PublisherDetail, "publisher_detail", "/publishers/<id>")
    # api.route(PublisherAuthorRelationship, "publisher_authors", "/publishers/<id>/relationships/persons")
