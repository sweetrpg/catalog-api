from setuptools import setup

# Metadata goes in setup.cfg. These are here for GitHub's dependency graph.
setup(
    name="sweetrpg-catalog-api",
    install_requires=[
        "analytics-python~=1.0",
        "blinker~=1.0",
        "dnspython~=2.0",
        "Flask-Caching~=2.0",
        "Flask-CORS~=5.0",
        "Flask-DotEnv~=0.1",
        "Flask-Migrate~=4.0",
        "Flask-MongoEngine~=1.0",
        "Flask-OAuthlib~=0.9",
        "Flask-REST-JSONAPI~=0.31",
        "Flask-Restful~=0.3",
        "Flask-Session~=0.8",
        "Flask-Swagger~=0.2",
        "Flask~=3.0",
        "hiredis~=3.0",
        "kanka~=0.1",
        "prometheus-flask-exporter~=0.23",
        "PyMongo~=4.0",
        "python-dateutil~=2.0",
        "python-dotenv~=1.0",
        "python-editor~=1.0",
        "PyYAML~=6.0",
        "requests~=2.0",
        "sentry-sdk[flask]~=2.0",
        "sweetrpg-api-core",
        "sweetrpg-catalog-objects",
        "sweetrpg-db",
        "sweetrpg-model-core",
        "urllib3~=2.0",
    ],
    extras_require={},
)
