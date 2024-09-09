from queue import Queue
import signal
import socket
import logging
import sys
import threading

from lottery.bet import Bet, has_won, load_bets, store_bets
from lottery.server_response import ServerResponse

READ_BUFFER_SIZE = 8192 # 8kB
U8_SIZE = 1
BATCH_SIZE_BYTES = U8_SIZE*2
END_NOTIFICATION = 'E'
TIMEOUT_WINNERS = 5

class LotteryCentral:
    def __init__(self, port, listen_backlog, max_clients):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._max_clients = max_clients

        self._threads = Queue() # Is thread safe
        self._winners_by_agency = {}

        self._lock_winners = threading.Lock()
        self._lock_persistence = threading.Lock()
        
        self._barrier = threading.Barrier(max_clients+1) # +1 for winners calculator thread
        self._winners_ready = threading.Condition()

    def run(self):
        """
        LotteryCentral that accept a new connections and establishes a
        communication with a lottery client. After client with communucation
        finishes, servers starts to accept new connections again
        """
        signal.signal(signal.SIGTERM, self.handle_SIGTERM)

        while True:
            winner_processor = threading.Thread(target=self.winners, args=())
            winner_processor.start()
            self._threads.put(winner_processor)
            max_clients = self._max_clients
            while max_clients > 0:
                client_sock = self.__accept_new_connection()
                client_thread = threading.Thread(target=self.__handle_client_connection, args=(client_sock,))
                client_thread.start()
                self._threads.put(client_thread)
                max_clients -= 1

            self.wait_and_join_clients(winner_processor)

    def wait_and_join_clients(self, winner_processor):
        while self._threads.qsize() > 0:
            thread = self._threads.get()
            thread.join()
        winner_processor.join()

    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket
        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        addr = client_sock.getpeername()
        try:
            maybe_req = self.process_bets(client_sock)
            logging.info(f'action: receive_message | result: success | ip: {addr[0]}')
            self._barrier.wait()

            with self._winners_ready:
                self._winners_ready.wait(timeout=TIMEOUT_WINNERS)

            self.send_winners(client_sock, maybe_req)
        except Exception as e:
            logging.error(f'action: receive_message | result: fail | ip: {addr[0]} | error: {e}')
            client_sock.close()
            self._barrier.wait()

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
        while not self._threads.empty():
            thread = self._threads.get()
            thread.join()
            logging.info("action: close_client_connection | result: success")
        self._server_socket.close()
        logging.info("action: close_server | result: success")
        sys.exit(0)

    def process_bets(self, client_sock):
        buffer = b''
        processed_chunk_size = False
        chunk_size = 0
        read = 0 # without considering the size
    
        while True: 
            data = client_sock.recv(READ_BUFFER_SIZE-read)
            
            buffer += data
            read += len(buffer)
            if read > 0 and not processed_chunk_size:
                if buffer[:U8_SIZE].decode('utf-8') == END_NOTIFICATION:
                    logging.info("action: apuesta_recibida | result: success | end")
                    return buffer[U8_SIZE:]
                
                if read >= BATCH_SIZE_BYTES:
                    chunk_size = int.from_bytes(buffer[:BATCH_SIZE_BYTES], byteorder='big')
                    buffer = buffer[BATCH_SIZE_BYTES:] 
                    read -= BATCH_SIZE_BYTES
                    processed_chunk_size = True
            if read < chunk_size: 
                continue
            
            try:
                chunk, buffer = self.parse_chunk(buffer, chunk_size)
                with self._lock_persistence:
                    store_bets(chunk)
                client_sock.sendall(ServerResponse.ok_bytes())
            except TypeError:
                logging.error("action: apuesta_recibida | result: fail | error: invalid_bet")
                client_sock.sendall(ServerResponse.error_bytes())

            processed_chunk_size = False
            read = len(buffer)
    
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
            chunk.append(Bet(**bet_values))
            
        logging.info("action: apuesta_recibida | result: success | cantidad: %d", len(chunk))
        return chunk, buffer
    
    def wait_for_winners_request(self, client_sock):
        while True:
            data = client_sock.recv(U8_SIZE)
            if data:
                return int.from_bytes(data, byteorder='big')
                
    def winners(self):
        self._barrier.wait()
        bets = load_bets()
        winners_by_agency = {}
        for b in bets:
            if has_won(b):
                if b.agency not in winners_by_agency:
                    winners_by_agency[b.agency] = set()
                winners_by_agency[b.agency].update([b.document])

        logging.info("action: sorteo | result: success")

        with self._lock_winners:
            self._winners_by_agency = winners_by_agency
        with self._winners_ready:
            self._winners_ready.notify_all()
        self._barrier.reset()

    def send_winners(self, client_sock, maybe_req):
        try:
            agency = None
            if len(maybe_req) == 0:
                agency = self.wait_for_winners_request(client_sock)
            else:
                agency = int.from_bytes(maybe_req, byteorder='big')
                
            logging.info("action: received_message | result: success | winner_request from agency: %d", agency)
            
            with self._lock_winners:
                winners = self._winners_by_agency.get(agency, set())
           
            bytes = [int(w).to_bytes(4, 'big') for w in winners]
            size = len(bytes).to_bytes(2, 'big')
            msg = size + b''.join(bytes)

            client_sock.sendall(msg)
            logging.info("action: send_winners | result: success | cantidad: %d", len(winners))
        except Exception as e:
            logging.error("action: send_winners | result: fail | error: %s", e)
