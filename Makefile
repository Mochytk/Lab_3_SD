.PHONY: all build up down clean docker-VM1 docker-VM2 docker-VM3 docker-VM4

all: build

build:
	docker-compose build

up:
	docker-compose up -d

up-pequeno:
	CSV_FILE=pedidos_pequeno.csv docker-compose up -d

up-mediano:
	CSV_FILE=pedidos_mediano.csv docker-compose up -d

up-grande:
	CSV_FILE=pedidos_grande.csv docker-compose up -d

down:
	docker-compose down

clean:
	docker-compose down -v
	rm -rf bin/

docker-VM1:
	docker-compose -f docker-compose.vm.yml up broker

docker-VM2:
	docker-compose -f docker-compose.vm.yml up gateway datanode1 cliente1

docker-VM3:
	docker-compose -f docker-compose.vm.yml up datanode2 cliente2

docker-VM4:
	docker-compose -f docker-compose.vm.yml up datanode3 cliente3
