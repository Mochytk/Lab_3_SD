# Laboratorio 3: Sistemas Distribuidos - DistriEats

**Integrantes (Grupo XX):**
- [Nombre Integrante 1] - [Rol]
- [Nombre Integrante 2] - [Rol]
- [Nombre Integrante 3] - [Rol]

## Descripción del Proyecto
Este proyecto simula "DistriEats", un ecosistema distribuido de gestión logística de entrega de comida. El sistema fue desarrollado en **Go** utilizando **gRPC** para la comunicación síncrona y estructurado para correr completamente orquestado con **Docker** y **Docker Compose**.

El ecosistema garantiza dos modelos de consistencia en simultáneo:
1. **Read Your Writes (RYW)** para los clientes finales (gracias a un Gateway que utiliza sesiones adherentes o *Sticky Sessions*).
2. **Consistencia Eventual** para la red de Datanodes (resolviendo actualizaciones concurrentes mediante un protocolo Gossip y Relojes Vectoriales).

---

## Prerrequisitos
- **Docker** (Docker Engine o Docker Desktop).
- **Make** instalado en tu sistema.

## Instrucciones de Ejecución

Para facilitar la corrección y orquestar las "Máquinas Virtuales" descritas en el enunciado, hemos provisto un archivo `Makefile` y un `docker-compose.yml`.

### 1. Despliegue y Construcción
Antes de iniciar, debes compilar el código y generar las imágenes de Docker. Ejecuta en la raíz del proyecto:
```bash
make build
```

### 2. Iniciar el Ecosistema Completo (Todas las Fases Automáticas)
Para desplegar todos los servicios puedes elegir el tamaño de la prueba (es decir, qué archivo `.csv` va a procesar el Productor). Tienes las siguientes opciones:

- Para usar el archivo pequeño (`pedidos_pequeno.csv`):
  ```bash
  make up-pequeno
  ```
- Para usar el archivo mediano (`pedidos_mediano.csv`):
  ```bash
  make up-mediano
  ```
- Para usar el archivo grande (`pedidos_grande.csv`):
  ```bash
  make up-grande
  ```
*(Si usas simplemente `make up`, intentará buscar por defecto `pedidos.csv`).*

---

### Ejecución Distribuida en 4 Máquinas Virtuales (MV1 - MV4)
Si vas a correr el laboratorio de manera verdaderamente distribuida a través de 4 máquinas virtuales diferentes (como indica el enunciado original), debes seguir estos pasos:

1. **Configurar las Direcciones IP**:
   Abre el archivo `docker-compose.yml` y actualiza los comandos de inicio de cada servicio reemplazando los nombres internos (`broker:50050`, `datanode1:50051`, etc.) por las **Direcciones IP reales** de las máquinas virtuales en tu red local.
   *Ejemplo*: Si el Broker (MV1) está en la IP `192.168.1.10`, en el Gateway de la MV2 debes cambiar `--broker=broker:50050` por `--broker=192.168.1.10:50050`. Haz lo mismo con los `--peers` de los datanodes y los `--datanodes` del broker/gateway.

2. **Copiar el Código**:
   Asegúrate de clonar/copiar todo este repositorio en las 4 máquinas virtuales, y ejecutar `make build` en cada una de ellas para generar la imagen de Docker.

3. **Ejecutar cada Entidad por Separado**:
   Entra a cada máquina virtual y ejecuta **únicamente** el comando `make` que le corresponde según la distribución solicitada:
   
   - **En MV1**: `make docker-VM1` *(Levanta el Broker Central / Productor de Eventos).*
   - **En MV2**: `make docker-VM2` *(Levanta el Gateway de Pedidos / Cliente Hambriento 1 / Datanode 1).*
   - **En MV3**: `make docker-VM3` *(Levanta el Cliente Hambriento 2 / Datanode 2).*
   - **En MV4**: `make docker-VM4` *(Levanta el Cliente Hambriento 3 / Datanode 3).*

*(Nota: En caso de probarlo de manera distribuida, para probar distintos archivos CSV debes inyectar la variable de entorno tú mismo, ej: `CSV_FILE=pedidos_pequeno.csv make docker-VM1` en la MV1).*

---

Una vez ejecutado este comando (ya sea en un solo host o en las 4 MVs):
1. Los servidores gRPC se inician de inmediato.
2. Tras 5 segundos, el **Broker (Productor)** comenzará a leer secuencialmente el archivo `broker/pedidos.csv` inyectando eventos cada 1-3 segundos y propagándolos a los Datanodes mediante _Broadcast_.
3. A los 10 segundos, los **Clientes Hambrientos** despertarán e intentarán consultar el estado base, emitirán un pedido, y rápidamente consultarán el estado de su pedido validando en consola la inmediatez e imprimiendo si la garantía RYW fue exitosa.

Puedes observar la ejecución de los contenedores y los flujos leyendo los logs en tiempo real:
```bash
docker compose logs -f
```

### 3. Fase de Caos y Tolerancia a Fallos (Opcional durante la simulación)
La rúbrica indica que se debe demostrar que el sistema no colapsa al caerse un nodo. Mientras el sistema se encuentra ejecutando (puedes verlo en los logs de `make up`), puedes detener de forma manual uno de los Datanodes abriendo otra terminal y ejecutando:

```bash
docker stop distrieats-datanode3
```
Verás que el Broker y el Gateway manejan el error y enrutan las solicitudes a los nodos restantes. Luego, puedes volver a iniciarlo:
```bash
docker start distrieats-datanode3
```
El nodo despertará sin estado, pero recuperará todo su historial tras unos segundos gracias a la comunicación asíncrona del **Protocolo Gossip**.

### 4. Convergencia Global y Cierre (Reporte Final)
Cuando el productor termine de leer todo el archivo `pedidos.csv`, se iniciará automáticamente un "Tiempo de Gracia" de **15 segundos**. Tras este lapso:
- El Broker recolectará el estado global convergido.
- Se recopilarán las confirmaciones exitosas del modelo Read Your Writes.
- Se generará un archivo consolidado localmente en el contenedor del Broker.

Para poder ver y extraer el archivo `Reporte.txt` autogenerado tras la convergencia, puedes ejecutar:
```bash
docker cp distrieats-broker:/app/Reporte.txt ./Reporte.txt
cat Reporte.txt
```

### 5. Detener y Limpiar el Entorno
Cuando termines de probar el laboratorio, asegúrate de apagar y limpiar la red de Docker:
```bash
make clean
```
