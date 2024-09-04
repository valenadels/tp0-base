import signal
import socket
import logging
import sys

from lottery.bet import Bet, has_won, load_bets, store_bets
from lottery.server_response import ServerResponse

READ_BUFFER_SIZE = 8192 # 8kB
U8_SIZE = 1
END = 'E'

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
        max_connections = 5

        while max_connections > 0:
            client_sock = self.__accept_new_connection()
            self._client_sockets.append(client_sock)
            self.__handle_client_connection(client_sock)
            max_connections -= 1
        
        logging.info("action: sorteo | result: success")

        winners_by_agency = self.winners()
        for a in self._client_sockets:
            self.send_winners(a, winners_by_agency)

        self._client_sockets.clear()
        
            
    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket
        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        addr = client_sock.getpeername()
        try:
            self.process_bets(client_sock)
            logging.info(f'action: receive_message | result: success | ip: {addr[0]}')
        except:
            logging.error(f'action: receive_message | result: fail | ip: {addr[0]}')
            self._client_sockets.remove(client_sock)
            client_sock.close()

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
        finished = False
        while not finished: 
            try:
                data = client_sock.recv(READ_BUFFER_SIZE-read)
            except BrokenPipeError | TimeoutError as e:
                raise e 
            
            buffer += data
            read += len(buffer)
            if read > 0 and not processed_chunk_size:
                first_byte = buffer[:batch_size_bytes]
                try: 
                    if first_byte.decode('utf-8') == END:
                        finished = True
                        break
                except:
                    pass

                chunk_size = int.from_bytes(first_byte, byteorder='big')
                buffer = buffer[batch_size_bytes:] 
                read -= batch_size_bytes
                processed_chunk_size = True
            if read < chunk_size: 
                continue
            
            try:
                chunk, buffer = self.parse_chunk(buffer, chunk_size)
                store_bets(chunk)
                client_sock.sendall(ServerResponse.ok_bytes())
            except BrokenPipeError as bp:
                raise bp
            except TypeError as e:
                try:
                    client_sock.sendall(ServerResponse.error_bytes())
                except Exception as ex:
                    raise ex

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
                raise e
            
        logging.info("action: apuesta_recibida | result: success | cantidad: %d", len(chunk))
        return chunk, buffer
    
    def wait_for_notification(self, client_sock):
        while True:
            try:
                data = client_sock.recv(U8_SIZE)
                if data:
                    logging.info("action: receive_message | result: success | data: %s", data)
                    break
            except BrokenPipeError as e:
                raise e
            
    def winners(self):
        bets = load_bets()
        winners_by_agency = {}
        for b in bets:
            if has_won(b):
                if b.agency not in winners_by_agency:
                    winners_by_agency[b.agency] = []
                winners_by_agency[b.agency].append(b.document)
        return winners_by_agency
    
    def send_winners(self, client_sock, winners_by_agency):
        try:
            agency = self.wait_for_request(client_sock)
            winners = winners_by_agency.get(agency, []) 
            bytes = [int(w).to_bytes(4, 'big') for w in winners]
            size = len(bytes).to_bytes(2, 'big')
            msg = size + b''.join(bytes)

            client_sock.sendall(msg)
            logging.info("action: send_winners | result: success | cantidad: %d", len(winners))
        except Exception as e:
            logging.error("action: send_winners | result: fail | error: %s", e)
    
    def wait_for_request(self, client_sock):
        while True:
            try:
                data = client_sock.recv(U8_SIZE)
                if data:
                    logging.info("action: receive_message | result: success | data: %s", data)
                    return int.from_bytes(data, 'big')
            except BrokenPipeError as e:
                raise e