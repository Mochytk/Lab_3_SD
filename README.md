# DistriEats - Sistema Distribuido de Pedidos

## Integrantes:
- Sebastian Olea 202273566-K
- Eduardo Rodriguez 202273593-7
- Sergio Rojas 202273619-4

## Información para el correcto funcionamiento del laboratorio:

Primero es necesario encontrarse en el directorio del Makefile en las 4 terminales, es decir en "../Grupo-1/Tarea3"
Ejecutar en todas las terminales de linux:
```bash
cd Grupo-1/Tarea3/
# Los siguientes 2 comandos solo si se realiza sudo make clean
export PATH="$PATH:$(go env GOPATH)/bin"
make build

'''
En caso de haber algún problema ejecutar **make build**, correr los siguientes comandos en todas las terminales:
'''
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
```
y volver a ejecutar **make build** en todas las terminales.

Luego de eso, levantar los servicios en las 4 terminales diferentes (dist001, dist002, dist003, dist004), en el siguiente orden:

```bash
# ==========================================
# MÁQUINA 1 (dist001) - Levantar Broker
# ==========================================
# Realizamos los exports para que el Broker conozca a los Datanodes
export DATANODES_ADDRESS=dist002.inf.santiago.usm.cl:50051,dist003.inf.santiago.usm.cl:50052,dist004.inf.santiago.usm.cl:50053

# (Opcional) Podemos elegir el CSV de prueba
export CSV_FILE=pedidos_pequeno.csv

# Levantamos el Broker
sudo -E make docker-VM1


# ==========================================
# MÁQUINA 2 (dist002) - Levantar Gateway, Datanode 1 y Cliente 1
# ==========================================
export BROKER_ADDRESS=dist001.inf.santiago.usm.cl:50050
export DATANODES_ADDRESS=dist002.inf.santiago.usm.cl:50051,dist003.inf.santiago.usm.cl:50052,dist004.inf.santiago.usm.cl:50053
export GATEWAY_ADDRESS=dist002.inf.santiago.usm.cl:50040
export PEERS_DN1=dist003.inf.santiago.usm.cl:50052,dist004.inf.santiago.usm.cl:50053

# Levantamos la VM2
sudo -E make docker-VM2


# ==========================================
# MÁQUINA 3 (dist003) - Levantar Datanode 2 y Cliente 2
# ==========================================
export GATEWAY_ADDRESS=dist002.inf.santiago.usm.cl:50040
export PEERS_DN2=dist002.inf.santiago.usm.cl:50051,dist004.inf.santiago.usm.cl:50053

# Levantamos la VM3
sudo -E make docker-VM3


# ==========================================
# MÁQUINA 4 (dist004) - Levantar Datanode 3 y Cliente 3
# ==========================================
export GATEWAY_ADDRESS=dist002.inf.santiago.usm.cl:50040
export PEERS_DN3=dist002.inf.santiago.usm.cl:50051,dist003.inf.santiago.usm.cl:50052

# Levantamos la VM4
sudo -E make docker-VM4
```

**En caso de que falle, hacer sudo make clean y repetir desde el primer paso para la Máquina 1.**

Para leer los logs en tiempo real si algún contenedor se ejecuta en segundo plano:
```bash
sudo docker-compose logs -f
```
