.PHONY: all build up down clean docker-VM1 docker-VM2 docker-VM3 docker-VM4

all: build

build:
	docker-compose build

up:
	docker-compose up -d

down:
	docker-compose down

clean:
	docker-compose down -v
	rm -rf bin/

docker-VM1:
	docker-compose up broker

docker-VM2:
	docker-compose up gateway datanode1 cliente1

docker-VM3:
	docker-compose up datanode2 cliente2

docker-VM4:
	docker-compose up datanode3 cliente3
