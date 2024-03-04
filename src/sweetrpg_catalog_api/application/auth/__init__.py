# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""Authn/z
"""

from flask_oauthlib.provider import OAuth2Provider
# from authlib.integrations.flask_client import OAuth
from flask import current_app
import logging
from sweetrpg_catalog_api.application import constants
import os


oauth = OAuth2Provider()
# oauth = OAuth()

@oauth.tokengetter
def _load_token(access_token=None, refresh_token=None):
    current_app.logger.debug("access_token: %s, refresh_token=%s", access_token, refresh_token)
    #     auth0 = current_app.auth0
    #     # auth0.authorize_access_token()
    #     resp = auth0.get(f'https://{os.environ[constants.AUTH0_DOMAIN]}/userinfo', token=access_token)
    #     current_app.logger.debug("resp: %s", resp)
    #     return { "token": access_token, "expires": 0 }
    return None

@oauth.tokensetter
def _save_token(token, request, *args, **kwargs):
    current_app.logger.debug("token: %s, request=%s", token, request)
    return None

@oauth.grantgetter
def _get_grant(x):
    current_app.logger.debug("x: %s", x)
    pass

@oauth.grantsetter
def _set_grant(x):
    current_app.logger.debug("x: %s", x)
    pass

@oauth.clientgetter
def _get_client(x):
    current_app.logger.debug("x: %s", x)
    pass

def _get_scope(resource, method):
    current_app.logger.debug("resource: %s, method: %s", resource, method)
