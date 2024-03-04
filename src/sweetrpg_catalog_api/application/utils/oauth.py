# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
OAuth functions
"""

import os
# from authlib.flask.client import OAuth
from authlib.integrations.flask_client import OAuth
from sweetrpg_catalog_api.application import constants
from flask import current_app


# oauth = OAuth(current_app)

# auth0 = oauth.register(
#     'auth0',
#     client_id=os.environ[constants.AUTH0_CLIENT_ID],
#     client_secret=os.environ[constants.AUTH0_CLIENT_SECRET],
#     api_base_url=os.environ[constants.AUTH0_DOMAIN],
#     access_token_url=os.environ[constants.AUTH0_DOMAIN] + '/oauth/token',
#     authorize_url=os.environ[constants.AUTH0_DOMAIN] + '/authorize',
#     client_kwargs={
#         # 'scope': 'openid',
#         'scope': 'openid profile email',
#     },
# )


# kanka = oauth.register(
#     'kanka',
#     client_id=os.environ[constants.KANKA_CLIENT_ID],
#     client_secret=os.environ[constants.KANKA_CLIENT_SECRET],
#     api_base_url=os.environ[constants.KANKA_DOMAIN],
#     access_token_url=os.environ[constants.KANKA_DOMAIN] + '/oauth/token',
#     authorize_url=os.environ[constants.KANKA_DOMAIN] + '/authorize',
#     client_kwargs={
#         # 'scope': 'openid',
#         'scope': 'openid profile email',
#     },
# )
