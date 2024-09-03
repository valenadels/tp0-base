package lottery

import (
	"encoding/binary"
	"errors"
	"io"
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
const END_OF_BETS = 1
const U8_LEN = 1
const U16_LEN = 2

// AgencyConfig Configuration used by the agency
type AgencyConfig struct {
	ID             string
	ServerAddress  string
	LoopAmount     int
	LoopPeriod     time.Duration
	MaxBatchAmount int
	BetReader 	   *BetReader
}

// Agency Entity that encapsulates how
type Agency struct {
	config AgencyConfig
	conn   net.Conn
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
		a.HandleSIGTERM(sigs)
		os.Exit(0)
	}()

	if a.createAgencySocket() != nil {
		a.closeAll(sigs)
	}

	if a.sendBets() != nil {
		a.closeAll(sigs)
	}

	if a.getWinners() != nil {
		a.closeAll(sigs)
	}

	if a.conn.Close() == nil {
		log.Infof("action: close_connection | result: success | client_id: %v", a.config.ID)
	}
}

// Close all connections and exit with code 1. 
// This function should be called when an error occurs in the agency.
func (a *Agency) closeAll(sigs chan os.Signal) {
	close(sigs)

	if a.config.BetReader != nil {
		a.config.BetReader.Close()
	}

	if a.conn != nil {
		a.conn.Close()
	}

	os.Exit(1)
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
		sendError := a.sendBatch(length, batch)
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

func (a *Agency) sendBatch(length int, batch []byte) error {
	for length > 0 {
		n, err := a.conn.Write(batch)
		if errors.Is(err, io.ErrClosedPipe) {
			return err
		}

		batch = batch[n:]
		length -= n
	}

	return nil
}

func (a *Agency) sendEndOfBets() error {
	msg := []byte{END_OF_BETS}
	for {
		n, err := a.conn.Write(msg)
		if err != nil {
			return err
		} else if n == U8_LEN {
			return nil
		}
	}
}

func (a *Agency) getWinners() error {
	requestErr := a.sendWinnersRequest()
	if requestErr != nil {
		return requestErr
	}

	buffer := make([]byte, READ_BUFFER_SIZE)
	var auxBuffer []byte
	winnersLen := uint16(0)
	bytesRead := 0 //without considering the length of the msg
	for {
		auxBuffer = buffer[bytesRead:]
		w, err := a.conn.Read(auxBuffer)
		if err.Error() == "EOF" { //server closed connection bc there is no more data
			break
		}else if err != nil {
			log.Criticalf("action: consulta_ganadores | result: fail | error: %v", err)
			return err
		}
		
		copy(buffer[bytesRead:], auxBuffer)

		if w >= U16_LEN && winnersLen == 0 {
			winnersLen = binary.BigEndian.Uint16(buffer)
			buffer = buffer[U16_LEN:]
			bytesRead -= U16_LEN
		}
		
		bytesRead += w
	
		if(bytesRead >= int(winnersLen)){
			buffer = parseWinners(buffer, winnersLen)
		}else if(bytesRead < READ_BUFFER_SIZE){
			continue
		}
	}

	log.Infof(`action: consulta_ganadores | result: success | cant_ganadores: ${CANT}`, winnersLen)
	return nil
}

func (a *Agency) sendWinnersRequest() error {
	agencyId, _ := strconv.Atoi(a.config.ID)
	msg := []byte{uint8(agencyId)}

	for {
		n, err := a.conn.Write(msg)
		if err != nil {
			return err
		} else if n == U8_LEN {
			return nil
		}
	}
}

func parseWinners(buffer []byte, winnersLen uint16) []byte {
	winners := make([]byte, winnersLen)
	var i uint16
	for i = 0; i < READ_BUFFER_SIZE && i < winnersLen; i++ {
		winners[i] = buffer[i]
	}

	return buffer[i:] //Todo VALEN ver si se resetea a todo 0
}

func (a *Agency) HandleSIGTERM(sigs chan os.Signal) {
	if a.conn != nil {
		err := a.conn.Close()
		if err == nil {
			log.Infof("action: close_connection | result: success | client_id: %v", a.config.ID)
		}
	}

	if a.config.BetReader != nil {
		a.config.BetReader.Close()
	}

	if sigs != nil {
		close(sigs)
		log.Infof("action: close_client | result: success | client_id: %v", a.config.ID)
	}
}