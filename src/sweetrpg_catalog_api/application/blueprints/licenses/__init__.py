# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""License routes.
"""

# from flask_rest_jsonapi import Api
from .manager import LicenseList, LicenseDetail, LicenseVolumeRelationship
from ...blueprints import api


def setup_routes(app):
    # app.logger.info("Registering license models...")
    # for model_name, model_info in license_model_info.items():
    #     APIData.add_model(model_name, model_info)

    app.logger.info("Setting up routes for Licenses API")
    # api = Api(app)
    api.route(LicenseList, "license_list", "/licenses/")
    api.route(LicenseDetail, "license_detail", "/licenses/<id>")
    api.route(LicenseVolumeRelationship, "license_authors", "/licenses/<id>/relationships/volumes")
