auth:
	cd ./orderhub-auth-service &&	make run

auth-doc:
	@echo poka ne gotovo, no sdelayu

auth-migrate:
	cd ./orderhub-auth-service &&	make migrate