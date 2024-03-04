# -*- coding: utf-8 -*-
__author__ = "Paul Schifferer <dm@sweetrpg.com>"
"""
"""

from datetime import datetime
from bson.timestamp import Timestamp


def to_datetime(value, attr=None, data=None, **kwargs):
    """Deserialize database value to Python datetime.
    :param any value:
    :param str attr:
    :param object data:
    :param dict kwargs:
    :return datetime.datetime: Python datetime object
    """
    print(f"to_datetime: value (parameter): {value}")
    if value is None:
        print(f"to_datetime: None")
        return None
    elif isinstance(value, Timestamp):
        print(f"to_datetime: Timestamp")
        value = value.as_datetime().timestamp()
    elif isinstance(value, datetime):
        print(f"to_datetime: datetime")
        return value
    elif isinstance(value, str):
        print(f"to_datetime: str")
        value = datetime.strptime(value)
    print(f"value (converted?): {value}")
    return datetime.fromtimestamp(float(value))

def to_timestamp(value, attr=None, obj=None, **kwargs):
    """Serialize object value to MongoDB timestamp.
    :param any value:
    :param str attr: The name of the attribute being serialized.
    :param object obj:
    :param dict kwargs:
    :return bson.timestamp.Timestamp: MongoDB Timestamp object
    """
    print(f"to_timestamp: value (parameter): {value}")
    if value is None:
        print(f"to_timestamp: None")
        return None
    return Timestamp(value.timestamp(), 0)
