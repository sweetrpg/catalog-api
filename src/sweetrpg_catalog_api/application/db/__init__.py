# -*- coding: utf-8 -*-
# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

from flask_mongoengine import MongoEngine


db = MongoEngine()

# def setup_database(app):
#     app.logger.info("Setting up database...")
#     global _client
#     _client = Connection(app.config["MONGO_URI"])
#     app.logger.debug("_client: %s", _client)
#     global db
#     db = Database(_client, app.config['DB_NAME'])
#     app.logger.debug("db: %s", db)

#     return db

# def register_models(app):
#     app.logger.info("Registering object mappings...")
#     _client.register([VolumeDocument])


# def setup_indexes(app, client):
#     app.logger.info("Setting up database indexes...")
#     client.volumes.create_index([('slug', pymongo.ASCENDING)], name='volume_slug', unique=True)
#     client.volumes.create_index([('name', pymongo.ASCENDING)], name='volume_name')
#     client.volumes.create_index([('system', pymongo.ASCENDING)], name='volume_system')
#     client.persons.create_index([('name', pymongo.ASCENDING)], name='author_name')
