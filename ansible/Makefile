include ../.env.production

deploy-server: install-collections
	ansible-playbook server.yml

deploy-postgres: install-collections
	ansible-playbook postgres.yml

deploy-service: install-collections
	ansible-playbook service.yml

deploy-migrations: install-collections
	ansible-playbook migrations.yml

install-collections:
	ansible-galaxy install -r requirements.yml
