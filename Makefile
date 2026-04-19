.PHONY: up down logs reset build ps

## Start all services
up:
	docker compose up --build -d

## Stop all services
down:
	docker compose down

## Follow logs (all services)
logs:
	docker compose logs -f

## Follow logs for a specific service: make logs-svc svc=user-service
logs-svc:
	docker compose logs -f $(svc)

## Rebuild a single service: make rebuild svc=user-service
rebuild:
	docker compose up --build -d $(svc)

## Stop everything and wipe all volumes (fresh start)
reset:
	docker compose down -v --remove-orphans

## Show running containers
ps:
	docker compose ps

## Open a psql shell into the user DB
db-user:
	docker exec -it postgres-user psql -U postgres -d userdb

## Open a psql shell into the account DB
db-account:
	docker exec -it postgres-account psql -U postgres -d accountdb

## Open Kafdrop (Kafka UI) in browser
kafka-ui:
	open http://localhost:9000

## Open Grafana in browser
grafana:
	open http://localhost:3000

## Open Mailhog in browser
mailhog:
	open http://localhost:8025

## Open Jaeger tracing UI
jaeger:
	open http://localhost:16686
