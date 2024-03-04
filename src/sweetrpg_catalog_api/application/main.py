# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""main.py

Creates a Flask app instance and registers various services and middleware.
"""

# import analytics
import os
from logging.config import dictConfig
from dotenv import load_dotenv, find_dotenv
from flask import Flask
from flask_migrate import Migrate
from flask_session import Session
from sentry_sdk.integrations.wsgi import SentryWsgiMiddleware
from werkzeug.middleware.proxy_fix import ProxyFix
from prometheus_flask_exporter import PrometheusMetrics
from sweetrpg_catalog_api.application import constants
from sweetrpg_catalog_api.application.cache import cache
import sweetrpg_catalog_api
from flask_swagger import swagger as Swagger

# from flask_swagger_ui import get_swaggerui_blueprint


ENV_FILE = find_dotenv()
if ENV_FILE:
    print(f"Loading environment from {ENV_FILE}...")
    load_dotenv(ENV_FILE)


def create_app(app_name=constants.APPLICATION_NAME):
    print("Configuring logging...")
    handlers = ["wsgi"]
    formatters = {
        "default": {
            "format": "[%(asctime)s] %(levelname)s %(module)s/%(funcName)s: %(message)s",
        }
    }
    handler_configs = {"wsgi": {"class": "logging.StreamHandler", "stream": "ext://flask.logging.wsgi_errors_stream", "formatter": "default"}}
    logstash_host = os.environ.get(constants.LOGSTASH_HOST)
    if logstash_host:
        handlers.append("logstash")
        handler_configs["logstash"] = {
            "class": "logstash_async.handler.AsynchronousLogstashHandler",
            "formatter": "logstash",
            "host": logstash_host,
            "port": int(os.environ[constants.LOGSTASH_PORT]),
            "database_path": "/tmp/sweetrpg_catalog_api_flask_logstash.db",
            "transport": "logstash_async.transport.BeatsTransport",
        }
        formatters["logstash"] = {
            "class": "logstash_async.formatter.FlaskLogstashFormatter",
            "metadata": {"beat": "sweetrpg-library-api"},
        }
    logging_config = {
        "version": 1,
        "formatters": formatters,
        "handlers": handler_configs,
        "root": {"level": os.environ.get(constants.LOG_LEVEL) or "INFO", "handlers": handlers},
    }
    # print(logging_config)
    dictConfig(logging_config)
    # print("Logging setup.")

    print("Creating app...")
    app = Flask(app_name)
    app.debug = app.config["DEBUG"]
    app.config.from_object("sweetrpg_catalog_api.application.config.BaseConfig")
    # print(app.config)
    # env = DotEnv(app)

    metrics = PrometheusMetrics(app)
    # static information as metric
    metrics.info("app_info", sweetrpg_catalog_api.__name__, version=sweetrpg_catalog_api.__version__, build=sweetrpg_catalog_api.__build__)

    swagger = Swagger(app)

    app.logger.info("Setting up proxy fix...")
    app.wsgi_app = ProxyFix(app.wsgi_app)

    app.logger.info("Setting up cache...")
    cache.init_app(app)

    # app.logger.info("Setting up cache...")
    # oauth.init_app(app)

    # app.logger.info("Setting up cache...")
    # analytics.write_key = app.config.get("SEGMENT_WRITE_KEY")
    # analytics.debug = app.config.get("DEBUG") or False

    app.logger.info("Setting up session manager...")
    session = Session(app)

    # cors = CORS(app, resources={r"/*": {"origins": "*"}})

    if not app.debug:
        app.logger.info("Setting up Sentry...")
        sentry = SentryWsgiMiddleware(app)

    from sweetrpg_catalog_api.application.blueprints import api
    from sweetrpg_catalog_api.application.auth import oauth

    # from authlib.integrations.flask_client import OAuth
    api.init_app(app)
    oauth.init_app(app)
    oauth.app = app
    api.oauth_manager(oauth)
    # oauth.scope_setter(get_scope)

    # _oauth = OAuth(app)
    # oauth.register('auth0',
    #                client_id=os.environ[constants.AUTH0_CLIENT_ID],
    #                client_secret=os.environ[constants.AUTH0_CLIENT_SECRET],
    #                api_base_url=os.environ[constants.AUTH0_DOMAIN],
    #                access_token_url=os.environ[constants.AUTH0_CALLBACK_URL],
    #                authorize_url=os.environ[constants.AUTH0_LOGIN_URL],
    #                client_kwargs={
    #                    'scope': 'openid profile email',
    #                })

    app.logger.info("Setting up endpoints...")

    # from sweetrpg_catalog_api.application.blueprints.volumes import blueprint as volumes_blueprint
    # app.register_blueprint(volumes_blueprint, url_prefix="/volumes")

    from sweetrpg_catalog_api.application.blueprints.licenses import setup_routes as setup_license_routes

    setup_license_routes(app)
    from sweetrpg_catalog_api.application.blueprints.volumes import setup_routes as setup_volume_routes

    setup_volume_routes(app)
    from sweetrpg_catalog_api.application.blueprints.contributions import setup_routes as setup_contrib_routes

    setup_contrib_routes(app)
    from sweetrpg_catalog_api.application.blueprints.persons import setup_routes as setup_person_routes

    setup_person_routes(app)
    from sweetrpg_catalog_api.application.blueprints.publishers import setup_routes as setup_publisher_routes

    setup_publisher_routes(app)
    from sweetrpg_catalog_api.application.blueprints.reviews import setup_routes as setup_review_routes

    setup_review_routes(app)
    from sweetrpg_catalog_api.application.blueprints.studios import setup_routes as setup_studio_routes

    setup_studio_routes(app)
    from sweetrpg_catalog_api.application.blueprints.systems import setup_routes as setup_system_routes

    setup_system_routes(app)

    from sweetrpg_api_core.blueprints.health import blueprint as health_blueprint
    from sweetrpg_catalog_api.application.utils.health import register_service_checks

    register_service_checks()
    app.register_blueprint(health_blueprint, url_prefix="/health")

    # swaggerui_blueprint = get_swaggerui_blueprint(
    #     constants.SWAGGER_URL,
    #     constants.API_URL,
    #     config={
    #         "app_name": "SweetRPG Library API",
    #     },
    # )
    # app.register_blueprint(swaggerui_blueprint, url_prefix=constants.SWAGGER_URL)

    app.logger.info("Setting up database...")
    from sweetrpg_catalog_api.application.db import db

    # print(db)
    db.init_app(app)
    app.logger.debug("Database: %s", db)
    migrate = Migrate(app, db)

    print(app.url_map)

    return app
