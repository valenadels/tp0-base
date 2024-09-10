package lottery

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

const READ_BUFFER_SIZE = 1024
const ACK_RESPONSE_SIZE = 1
const END_OF_BETS = 'E'
const U8_LEN = 1
const U16_LEN = 2
const DOCUMENT_SIZE_B = 4

// AgencyConfig Configuration used by the agency
type AgencyConfig struct {
	ID             string
	ServerAddress  string
	LoopAmount     int
	LoopPeriod     time.Duration
	MaxBatchAmount int
	BetReader      *BetReader
}

// Agency Entity that encapsulates how
type Agency struct {
	config AgencyConfig
	conn   net.Conn
	finished bool
}

// NewAgency Initializes a new agency receiving the configuration
// as a parameter
func NewAgency(config AgencyConfig) *Agency {
	agency := &Agency{
		config: config,
	}
	return agency
}

// createAgencySocket Initializes agency socket. In case of
// failure, error is printed in stdout/stderr and error is returned
func (a *Agency) createAgencySocket() error {
	conn, err := net.Dial("tcp", a.config.ServerAddress)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			a.config.ID,
			err,
		)
		return err
	}
	a.conn = conn
	return nil
}

func (a *Agency) StartAgency() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	go func() {
		<-sigs
		a.HandleSIGTERM()
	}()

    funcs := []func() error{a.createAgencySocket, a.sendBets, a.getWinners}
    for _, fn := range funcs {
        if fn() != nil {
            break
        }
    }

	a.closeAll()
}

// Close all connections 
// This function should be called when an error occurs in the agency.
func (a *Agency) closeAll() {
	if a.config.BetReader != nil {
		a.config.BetReader.Close()
	}

	if a.conn != nil {
		a.conn.Close()
		log.Infof("action: close_connection | result: success | client_id: %v", a.config.ID)
	}
}

func (a *Agency) sendBets() error {
	a.config.BetReader = NewBetReader(a.config.ID, a.config.MaxBatchAmount)
	if a.config.BetReader == nil {
		return errors.New("error creating bet reader")
	}

	batchNumber := 0
	for {
		batch, err := a.config.BetReader.ReadNextBatch()
		if err != nil {
			log.Criticalf(
				"action: read_file | result: fail | agency_id: %v | error: %v",
				a.config.ID,
				err,
			)
			return err
		}

		if len(batch) == 0 {
			break
		}

		batchNumber++
		batch = addBatchBytesLen(batch)
		length := len(batch)
		sendError := SendAll(a.conn, batch)
		if sendError != nil {
			log.Criticalf(`action: apuestas_enviadas | result: fail | bytes: %v | batch: %v | error: %v`, length, batchNumber, sendError)
			return sendError
		}

		readAckError := a.readAckResponse()
		if readAckError != nil {
			log.Criticalf(`action: apuestas_enviadas | result: fail | bytes: %v | batch: %v | error: %v`, length, batchNumber, readAckError)
			return readAckError
		}

		log.Infof(`action: apuestas_enviadas | result: success | bytes: %v | batch: %v`, length, batchNumber)
	}

	endErr := a.sendEndOfBets()
	a.config.BetReader.Close()
	a.config.BetReader = nil

	return endErr
}

func addBatchBytesLen(batch []byte) []byte {
	batchSize := uint16(len(batch))
	batchWithLength := []byte{
		byte(batchSize >> 8),
		byte(batchSize & 0xff),
	}

	return append(batchWithLength, batch...)
}

func (a *Agency) readAckResponse() error {
	ack := make([]byte, ACK_RESPONSE_SIZE)
	for{
		n, err := a.conn.Read(ack)
		if err != nil {
			return err
		}

		if n == ACK_RESPONSE_SIZE {
			return nil
		}
	}
}

func (a *Agency) sendEndOfBets() error {
	msg := []byte{END_OF_BETS}
	return SendAll(a.conn, msg)
}

func (a *Agency) getWinners() error {
	requestErr := a.sendWinnersRequest()
	if requestErr != nil {
		return requestErr
	}

	buffer := make([]byte, READ_BUFFER_SIZE)
	winnersLen := uint16(0)
	bytesRead := 0 //without considering the length of the msg
	for {
		n, err := a.conn.Read(buffer[bytesRead:])
		if err != nil {
			log.Criticalf("action: consulta_ganadores | result: fail | error: %v", err)
			return err
		}

		if n >= U16_LEN && winnersLen == 0 {
			winnersLen = binary.BigEndian.Uint16(buffer)
			buffer = buffer[U16_LEN:]
			bytesRead -= U16_LEN
		}
		
		bytesRead += n
	
		if(bytesRead == int(winnersLen*DOCUMENT_SIZE_B)){
			parseWinners(buffer, winnersLen)
			break
		}else if(bytesRead < READ_BUFFER_SIZE){
			continue
		}else{
			buffer = parseWinners(buffer, winnersLen)
		}
	}

	log.Infof(`action: consulta_ganadores | result: success | cant_ganadores: %v`, winnersLen)
	return nil
}

func (a *Agency) sendWinnersRequest() error {
	agencyId, _ := strconv.Atoi(a.config.ID)
	msg := []byte{uint8(agencyId)}
	return SendAll(a.conn, msg)
}

func parseWinners(buffer []byte, winnersLen uint16) []byte {
	var i uint16
	for i = 0; i < winnersLen && i < READ_BUFFER_SIZE; i++ {
		w_bytes := buffer[i*DOCUMENT_SIZE_B : (i+1)*DOCUMENT_SIZE_B]
		winner := binary.BigEndian.Uint32(w_bytes)
		log.Infof("action: winners_received | result: success | winners: %v", winner)
	}

	return buffer[i*DOCUMENT_SIZE_B:] 
}

func (a *Agency) HandleSIGTERM() {
	if a.conn != nil {
		err := a.conn.Close()
		if err == nil {
			log.Infof("action: close_connection | result: success | client_id: %v", a.config.ID)
		}
	}
}
