.PHONY: docs
init:
	pip install -r requirements/dev.txt
test:
	# This runs all of the tests.
	detox
ci:
	cd src && pytest ../tests --junitxml=report.xml

test-readme:
	python setup.py check --restructuredtext --strict && ([ $$? -eq 0 ] && echo "README.rst and HISTORY.rst ok") || echo "Invalid markup in README.rst or HISTORY.rst!"

flake8:
	flake8 --ignore=E501,F401,E128,E402,E731,F821 sweetrpg_catalog_api

coverage:
	pytest --cov-config .coveragerc --verbose --cov-report term --cov-report xml --cov=sweetrpg_catalog_api tests

publish:
	pip install 'twine>=5.1.0'
	python setup.py sdist bdist_wheel
	twine upload dist/*
	rm -fr build dist .egg sweetrpg_catalog_api.egg-info

docs:
	cd docs && make html
	@echo "\033[95m\n\nBuild successful! View the docs homepage at docs/_build/html/index.html.\n\033[0m"
