import signal
import socket
import logging
import sys

from lottery.bet import Bet, store_bets

READ_BUFFER_SIZE = 1024
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
        try:
            bets = self.read_bets_from_socket(client_sock)
            addr = client_sock.getpeername()
            logging.info(f'action: receive_message | result: success | ip: {addr[0]} | msg: {bets}')
            store_bets(bets)
            logging.info(f'action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}')
        except OSError as e:
            logging.error("action: receive_message | result: fail | error: {e}")
        finally:
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

    def read_bets_from_socket(self, client_sock):
        bet_fields = Bet.get_fields()
        buffer = b''
        bets = []

        while True:
            data = client_sock.recv(READ_BUFFER_SIZE)
            buffer += data
            bets += self.parse_bet(bet_fields.copy, buffer) #TODO VALEN ver si anda el copy

        return bets

    def parse_bet(self, bet_fields, buffer):
        bet_values = {}
        while bet_fields and len(buffer) >= U8_SIZE:
                length_data = int.from_bytes(buffer[:U8_SIZE], byteorder='big')
                buffer = buffer[U8_SIZE:] 

                if len(buffer) < length_data: 
                    break  

                field_data, buffer = buffer[:length_data], buffer[length_data:]
                bet_values[bet_fields.pop(0)] = field_data.decode('utf-8')
                
        return Bet(**bet_values)