# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

import jinja2
from flask import Blueprint, request, render_template, session, jsonify, current_app
from werkzeug.exceptions import HTTPException

from sweetrpg_catalog_api.application import constants


# from sweetrpg_catalog_api.application.models import constants as model_constants
# from .. import render_page, requires_auth
# from sweetrpg_catalog_api.application.models.user import User
# from sweetrpg_catalog_api.application.utils.user import has_role


# def requires_auth(f):
#     @wraps(f)
#     def _check_auth(*args, **kwargs):
#         if constants.PROFILE_KEY not in session:
#             return redirect('/auth/login')
#         return f(*args, **kwargs)

#     return _check_auth


# def user_info(f):
#     @wraps(f)
#     def decorated(*args, **kwargs):
#         if constants.PROFILE_KEY in session:
#             kwargs.update({
#                 'userinfo': session[constants.PROFILE_KEY],
#             })
#         return f(*args, **kwargs)

#     return decorated


def error_page(message, code):
    context = {
        "code": code,
        "message": message,
    }
    try:
        return render_page(f"errors/{code}.json")
    except jinja2.TemplateNotFound:
        return render_page("errors/error.json", context)


def render_page(page, context={}):

    show_cookie_message = True
    if request.cookies.get("cookies-accepted"):
        show_cookie_message = False

    userinfo = session.get(constants.PROFILE_KEY)
    if userinfo:
        context.update(
            {
                "showCookieMessage": show_cookie_message,
                "userinfo": userinfo,
            }
        )
    print(f"context: {context}")

    return render_template(page, **context)


class UserAuthorizationException(Exception):
    def __init__(self, reason: str):
        self.reason = reason


# def _check_user(role_name: str):
#     user_id = session.get(constants.CURRENT_USER_ID)
#     if user_id:
#         user = User.query.filter_by(id=user_id).first()
#         if user:
#             if has_role(user, role_name):
#                 return user

#             raise UserAuthorizationException('insufficient permissions')

#         raise UserAuthorizationException('user not found')

#     raise UserAuthorizationException('no user in session')


# def admin_required(f):
#     @wraps(f)
#     def _get_user(*args, **kwargs):
#         try:
#             user = _check_user(model_constants.ROLE_ADMIN)
#             return f(user, *args, **kwargs)
#         except UserAuthorizationException as e:
#             return jsonify({
#                 'error': "Unauthorized; " + e.reason
#             }), 401

#     return _get_user


# def user_required(f):
#     @wraps(f)
#     def _get_user(*args, **kwargs):
#         try:
#             user = _check_user(model_constants.ROLE_USER)
#             return f(user, *args, **kwargs)
#         except UserAuthorizationException as e:
#             return jsonify({
#                 'error': "Unauthorized; " + e.reason
#             }), 401

#     return _get_user


# def user_optional(f):
#     @wraps(f)
#     def _get_user(*args, **kwargs):
#         try:
#             user = _check_user(model_constants.ROLE_USER)
#             return f(user, *args, **kwargs)
#         except UserAuthorizationException as e:
#             return f(None, *args, **kwargs)

#     return _get_user


blueprint = Blueprint("api", __name__)


@blueprint.errorhandler(Exception)
def error_handler(ex):
    current_app.logger.exception(f"Exception caught: {ex}")
    response = jsonify(message=str(ex))
    response.status_code = ex.code if isinstance(ex, HTTPException) else 500
    return response


@blueprint.before_request
def log_request_headers():
    current_app.logger.info(f"Request headers: {request.headers}")


@blueprint.after_request
def log_response_headers(r):
    current_app.logger.info(f"Response headers: {r.headers}")
    return r


from flask_rest_jsonapi import Api

api = Api()

# from sweetrpg_catalog_api.application.blueprints.api.common import game_systems, utils
# from sweetrpg_catalog_api.application.blueprints.api.initiative import encounters, groups
from sweetrpg_catalog_api.application.blueprints import volumes
from sweetrpg_catalog_api.application.blueprints import persons
