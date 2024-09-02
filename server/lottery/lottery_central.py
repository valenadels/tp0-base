import signal
import socket
import logging
import sys

from lottery.bet import Bet, store_bets
from lottery.server_response import ServerResponse

READ_BUFFER_SIZE = 8192 # 8kB
U8_SIZE = 1

class LotteryCentral:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._client_sockets = []

    def run(self):
        """
        LotteryCentral that accept a new connections and establishes a
        communication with a lottery client. After client with communucation
        finishes, servers starts to accept new connections again
        """
        signal.signal(signal.SIGTERM, self.handle_SIGTERM)
        while True:
            client_sock = self.__accept_new_connection()
            self._client_sockets.append(client_sock)
            self.__handle_client_connection(client_sock)

    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket
        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        addr = client_sock.getpeername()
        self.process_bets(client_sock)
        logging.info(f'action: receive_message | result: success | ip: {addr[0]}')
        client_sock.close()
        self._client_sockets.remove(client_sock)

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """

        # Connection arrived
        logging.info('action: accept_connections | result: in_progress')
        c, addr = self._server_socket.accept()
        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        return c
    
    def handle_SIGTERM(self, signum, frame):
        for client_sock in self._client_sockets:
            client_sock.close()
            logging.info("action: close_client_connection | result: success")
        self._server_socket.close()
        logging.info("action: close_server | result: success")
        sys.exit(0)

    def process_bets(self, client_sock):
        buffer = b''
        processed_chunk_size = False
        chunk_size = 0
        batch_size_bytes = U8_SIZE*2
        read = 0
        timeout = 3
        while timeout > 0:
            try:
                data = client_sock.recv(READ_BUFFER_SIZE-read)
                if not data:
                    timeout -= 1
            except BrokenPipeError | TimeoutError:
                logging.info("action: receive_message | result: finished connection")
                return 
            
            buffer += data
            read += len(buffer)
            if read > 0 and not processed_chunk_size:
                chunk_size = int.from_bytes(buffer[:batch_size_bytes], byteorder='big')
                buffer = buffer[batch_size_bytes:] 
                read -= batch_size_bytes
                processed_chunk_size = True
            if read < chunk_size: 
                continue
            
            try:
                chunk, buffer = self.parse_chunk(buffer, chunk_size)
                store_bets(chunk)
                client_sock.sendall(ServerResponse.ok_bytes())
            except BrokenPipeError:
                logging.error("action: send_message | result: finished connection")
                return
            except Exception as e:
                logging.error("action: receive_message | result: fail | error: %s", e)
                try:
                    client_sock.sendall(ServerResponse.error_bytes())
                except BrokenPipeError:
                    logging.error("action: send_message | result: finished connection")
                    return

            processed_chunk_size = False
            read = 0

    def parse_chunk(self, buffer, chunk_size):
        chunk = []

        while chunk_size > 0:
            bet_fields = Bet.get_fields()
            bet_values = {}
            while bet_fields:
                if len(buffer) >= U8_SIZE:
                    length_data = int.from_bytes(buffer[:U8_SIZE], byteorder='big')
                    buffer = buffer[U8_SIZE:] 
                    chunk_size -= U8_SIZE

                    field_data, buffer = buffer[:length_data], buffer[length_data:]
                    bet_values[bet_fields.pop(0)] = field_data.decode('utf-8')
                    chunk_size -= length_data
            
            try:
                chunk.append(Bet(**bet_values))
            except TypeError as e:
                logging.info("action: apuesta_recibida | result: fail | cantidad: %d", len(chunk))
                return e
            
        logging.info("action: apuesta_recibida | result: success | cantidad: %d", len(chunk))
        return chunk, buffer