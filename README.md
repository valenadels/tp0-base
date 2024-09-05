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
    En caso de elegir otro nombre para el archivo, se debe modificar en el Makefile `docker-compose-dev.yaml` por el nombre elegido.

- ej2: En este item se modificó el docker compose para que cambios en los archivos de configuración (config.ini y config.yaml) no requieran un nuevo build. Esto se hizo utilizando volúmenes en el docker-compose. Para ejecutarlo, se debe correr el comando `make docker-compose-up`.

- ej3: Para ejecutar el script del ej3, ejecutar: `./validar-echo-server.sh` en la raiz del proyecto.

- ej4: Se modifica el cliente y el servidor para terminar de forma GRACEFUL. Para ejecutarlo, se debe correr el comando `make docker-compose-up` y luego si se hace un 
`docker-compose-down`, docker envía SIGTERM y se podrá ver que los containers se detienen de forma correcta. 
Se eliminó el flag -t para que no tenga un timeout que fuerce el fin de la ejecución.

- ej5: En este ejercicio, se modificó la lógica para representar una lotería. En el archivo docker compose se definen como variables de entorno los campos que representan la apuesta de una persona: nombre, apellido, DNI, nacimiento, numero apostado (en adelante 'número'). Ej: NOMBRE=Santiago Lionel, APELLIDO=Lorca, DOCUMENTO=30904465, NACIMIENTO=1999-03-17 y NUMERO=7574.


- ej6, ej7 y ej8: Para el correcto funcionamiento de los mismos, se debe descomprimir el archivo .zip que se encuentra en `.data` en el mismo directorio y luego, como siempre, ejecutar el comando `make docker-compose-up` para que comience a correr.

### Protocolo Parte 2
### Ejercicio 5
Se optó por un protocolo en el cual, los mensajes son de longitud variable. Para ello, se envía primero la longitud del mensaje (de un tamaño predefinido de uint8) y luego el campo en sí. Es decir:
`<SIZE_ID>AGENCY_ID<SIZE_NOMBRE>NOMBRE<SIZE_APELLIDO>APELLIDO<SIZE_DOCUMENTO>DOCUMENTO<SIZE_NACIMIENTO>NACIMIENO<SIZE_NUMERO>NUMERO`

Por ejemplo, si NOMBRE = "juan", el size de NOMBRE es 4 bytes, por lo que quedaría: `4juan`.

Los campos son todos strings. Al decodificarlos, el servidor los lee como utf-8 en big endian.

El protocolo de capa de transporte utilizado es TCP, por lo tanto, como en este ejercicio solo se envía una apuesta, decidí que con el ACK interno de TCP es suficiente para asegurar que el mensaje llegó correctamente, por lo que no hay un mensaje de confirmación extra del servidor. 

Se implementaron métodos para enviar y recibir los mensajes por los sockets que sean short-read y short-write safe.

### Ejercicio 6
Para este ejercicio, se continúa con el protocolo definido en el ítem anterior, pero se agregan algunas definiciones más.

Por enunciado:
```
La cantidad máxima de apuestas dentro de cada batch debe ser configurable desde config.yaml. Respetar la clave batch: maxAmount, pero modificar el valor por defecto de modo tal que los paquetes no excedan los 8kB. 
```

Por lo tanto, se modificó el archivo `config.yaml` para modificar la clave `batch` con el valor `maxAmount: 133`. Llegué a 133 calculando el máximo de apuestas que se pueden enviar sin exceder los 8kB, considerando el peor caso, en el que todos los campos tienen la longitud máxima posible (tomando como referencia los datasets provistos).

Se asume que la cantidad de clientes es limitada y es 5.

El envío de los batches se da de la siguiente manera:
- El cliente arma un mensaje que contiene:
    - La cantidad de bytes del batch en uint16.
    - Las apuestas en sí en el formato definido en el ejercicio anterior.
- El servidor cada vez que guarda un batch, envía un mensaje de 1Byte de confirmación que consiste en 1 (OK) o 0 (ERROR). Se envia ERROR cuando ocurre un problema al parsear los datos. Por ejemplo, porque el tipo de dato esperado al decodificar no es el que se recibe.
- El cliente espera al mensaje de confirmación antes de enviar el siguiente batch. (Sigue enviando por más que reciba ERROR).

### Ejercicio 7
Cuando el cliente (la agencia) termina de mandar todos los batches y recibe la última confirmación, envía un mensaje END al servidor. Este consiste en un mensaje de 1Byte que contiene el valor 'E'.

Recién cuando todas las agencias finalizaron, el servidor deja de procesar batches y procede a sortear los números ganadores. 

Las agencias envían un mensaje de consulta de ganadores compuesto por un uint8 que contiene el ID de la agencia. El servidor responde con un mensaje que contiene los DNIs de los ganadores de la agencia en cuestión.

### Mecanismo de Sincronización Parte 3

En este ejercicio, se modificó la implementación para permitir procesar varios clientes a la vez. Para ello, se utilizó la biblioteca `threading` de Python. Sin embargo, hubiera sido mejor utilizar `multiprocessing` para evitar el cuello de botella generado por el GIL de Python.
Se explican a continuación los mecanismos de sincronización y concurrencia utilizados:
- Threads: Cada cliente se ejecuta en un thread distinto. Además, hay un thread que se encarga exclusivamente de cargar las apuestas (load_bets) y realizar el sorteo. Al ser el único que se encarga de esto, no fue necesario utilizar mecanismos de sincronización sobre el acceso a las apuestas.
- Locks: 
    - Se utilizó un lock para sincronizar el acceso a los ganadores. Los ganadores se almacenan en una variable que es un Hashmap de Sets de DNIs. Esto permite que si se corren los clientes más de una vez con los mismos archivos de data, no se dupliquen los ganadores.
    - Otro lock se utilizó para sincronizar el acceso a la persistencia de las apuestas (store_bets).
- Barriers: Se utilizó una barrera para asegurar que todos los threads avancen al sorteo sólo cuando todos los clientes terminaron de enviar sus apuestas.

- Conditions: Se empleó una variable de condicion para que cada thread dedicado a cada cliente espere a que el thread que calcula los ganadores, termine de cargar las apuestas y finalice el sorteo. Surgió el problema de manejar el posible caso de una "señal perdida" que ocurriría si se realiza el notify_all() antes del wait(). Sin embargo, la probabilidad de que esto suceda es altamente baja ya que antes del notify, ese hilo realiza varias operaciones que toman tiempo. De todos modos, se optó por setear un timeout de 5 segundos en el wait() para evitar que el hilo quede esperando indefinidamente si se diera esta situación.

- Queue: Se usa para guardar los hilos para su posterior join.

Por último, como aclaración, se modificó el método `run` para que corra el servidor indefinidamente, pero procese de a 5 clientes por vez.