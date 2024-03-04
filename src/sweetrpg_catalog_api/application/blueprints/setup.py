# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

from sweetrpg_library_objects.db.contribution.document import ContributionDocument
from sweetrpg_library_objects.db.person.document import PersonDocument
from sweetrpg_library_objects.db.publisher.document import PublisherDocument
from sweetrpg_library_objects.db.review.document import ReviewDocument
from sweetrpg_library_objects.db.studio.document import StudioDocument
from sweetrpg_library_objects.db.system.document import SystemDocument
from sweetrpg_library_objects.db.volume.document import VolumeDocument
from sweetrpg_library_objects.model.contribution import Contribution
from sweetrpg_library_objects.model.person import Person
from sweetrpg_library_objects.model.publisher import Publisher
from sweetrpg_library_objects.model.review import Review
from sweetrpg_library_objects.model.studio import Studio
from sweetrpg_library_objects.model.system import System
from sweetrpg_library_objects.model.volume import Volume

model_info = {
    "person": {
        "model": Person,
        # "schema": PersonSchema,
        "document": PersonDocument,
        "type": "person",
        "collection": "persons",
        "properties": {"volume_ids": "volume"},
    },
    "contribution": {
        "model": Contribution,
        # "schema": ContributionSchema,
        "document": ContributionDocument,
        "type": "contribution",
        "collection": "contributions",
        "properties": {"volume_ids": "volume", "person_ids": "person"},
    },
    "volume": {
        "model": Volume,
        # "schema": VolumeSchema,
        "document": VolumeDocument,
        "type": "volume",
        "collection": "volumes",
        "properties": {"persons": "person", },
    },
    "publisher": {
        "model": Publisher,
        # "schema": PublisherSchema,
        "document": PublisherDocument,
        "type": "publisher",
        "collection": "publishers",
        "properties": {},
    },
    "review": {
        "model": Review,
        # "schema": ReviewSchema,
        "document": ReviewDocument,
        "type": "review",
        "collection": "reviews",
        "properties": {"volumes": "volume"},
    },
    "studio": {
        "model": Studio,
        # "schema": StudioSchema,
        "document": StudioDocument,
        "type": "studio",
        "collection": "studios",
        "properties": {"persons": "person"},
    },
    "system": {
        "model": System,
        # "schema": SystemSchema,
        "document": SystemDocument,
        "type": "system",
        "collection": "systems",
        "properties": {},
    },
}
