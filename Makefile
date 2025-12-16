test:
	docker-compose up --build

clean:
	docker-compose down -v
	docker system prune -f

logs-server:
	docker-compose logs -f server

logs-k6:
	docker-compose logs -f k6-client

results:
	docker-compose exec k6-client cat /results/summary.json