# TP0: Docker + Comunicaciones + Concurrencia

## Instrucciones de uso
El repositorio cuenta con un **Makefile** que posee encapsulado diferentes comandos utilizados recurrentemente en el proyecto en forma de targets. Los targets se ejecutan mediante la invocación de:

* **make \<target\>**:
Los target imprescindibles para iniciar y detener el sistema son **docker-compose-up** y **docker-compose-down**, siendo los restantes targets de utilidad para el proceso de _debugging_ y _troubleshooting_.

Los targets disponibles son:
* **docker-compose-up**: Inicializa el ambiente de desarrollo (buildear docker images del servidor y cliente, inicializar la red a utilizar por docker, etc.) y arranca los containers de las aplicaciones que componen el proyecto.
* **docker-compose-down**: Realiza un `docker-compose stop` para detener los containers asociados al compose y luego realiza un `docker-compose down` para destruir todos los recursos asociados al proyecto que fueron inicializados. Se recomienda ejecutar este comando al finalizar cada ejecución para evitar que el disco de la máquina host se llene.
* **docker-compose-logs**: Permite ver los logs actuales del proyecto. Acompañar con `grep` para lograr ver mensajes de una aplicación específica dentro del compose.
* **docker-image**: Buildea las imágenes a ser utilizadas tanto en el servidor como en el cliente. Este target es utilizado por **docker-compose-up**, por lo cual se lo puede utilizar para testear nuevos cambios en las imágenes antes de arrancar el proyecto.
* **build**: Compila la aplicación cliente para ejecución en el _host_ en lugar de en docker. La compilación de esta forma es mucho más rápida pero requiere tener el entorno de Golang instalado en la máquina _host_.

## Resolución de ejercicios
Cada ejercicio (del 1 al 8) se encuentra en una rama separada, llamada respectivamente ej<n°>.

### Ejecución
- ej1: Para generar el compose ejecutar: `./generar-compose.sh <nombre.yaml> <uintClientes>` en la raiz del proyecto. Por ejemplo:
    ```bash
    ./generar-compose.sh docker-compose-dev.yaml 5
    ```
    En caso de elegir otro nombre para el archivo, se debe modificar `docker-compose-dev.yaml` por el nombre elegido en el Makefile.

- ej2: En este item se modificó el docker compose para que cambios en los archivos de configuración (config.ini y config.yaml) no requiera un nuevo build. Esto se hizo utilizando volúmenes en el docker-compose. Para ejecutarlo, se debe correr el comando `make docker-compose-up`.

- ej3: Para ejecutar el script del ej3, ejecutar: `./validar-echo-server.sh` en la raiz del proyecto.

- ej4: Se modifica el cliente y el servidor para terminar de forma GRACEFUL. Para ejecutarlo, se debe correr el comando `make docker-compose-up` y luego si se hace un 
`docker-compose down`, docker envía SIGTERM y se podrá ver que los containers se detienen de forma correcta. 
Se eliminó el flag -t para que no tenga un timeout que fuerce el fin de la ejecución.

- ej5: En este ejercicio, se modificó la lógica para representar una lotería. En el archivo docker compose se definen como variables de entorno los campos que representan la apuesta de una persona: nombre, apellido, DNI, nacimiento, numero apostado (en adelante 'número'). Ej: NOMBRE=Santiago Lionel, APELLIDO=Lorca, DOCUMENTO=30904465, NACIMIENTO=1999-03-17 y NUMERO=7574.


- ej6, ej7 y ej8: Para el correcto funcionamiento de los mismos, se debe descomprimir el archivo .zip que se encuentra en `.data` en el mismo directorio y luego, como siempre, ejecutar el comando `make docker-compose-up` para que comience a correr.

### Protocolo