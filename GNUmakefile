default: testacc

# Define a prerequisite for the testacc target
testacc: docker-up

# Define a target to run `docker-compose up`
.PHONY: docker-up
docker-up:
	docker compose up -d

# Run acceptance tests
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m -count=1

# Define a target to stop the Docker containers
.PHONY: docker-down
docker-down:
	docker compose down -v