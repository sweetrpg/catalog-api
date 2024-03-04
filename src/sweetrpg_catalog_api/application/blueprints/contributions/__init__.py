# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

from .manager import ContributionList, ContributionDetail, ContributionVolumeRelationship, \
    ContributionPersonRelationship
from ...blueprints import api


def setup_routes(app):
    # app.logger.info("Registering contribution models...")
    # for model_name, model_info in person_model_info.items():
    #     APIData.add_model(model_name, model_info)

    app.logger.info("Setting up routes for Contributions API")
    # api = Api(app)
    api.route(ContributionList, "contribution_list", "/contributions/")
    api.route(ContributionDetail, "contribution_detail", "/contributions/<id>")
    api.route(ContributionVolumeRelationship, "contribution_volumes", "/contributions/<id>/relationships/volumes")
    api.route(ContributionPersonRelationship, "contribution_persons", "/contributions/<id>/relationships/persons")
