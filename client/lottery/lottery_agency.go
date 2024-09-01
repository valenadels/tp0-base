package lottery

import (
	"errors"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

const READ_BUFFER_SIZE = 1024
const RESPONSE_SIZE = 1

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
func (c *Agency) createAgencySocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
		return err
	}
	c.conn = conn
	return nil
}

func (c *Agency) StartAgency() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	go func() {
		<-sigs
		c.HandleSIGTERM(sigs)
		os.Exit(0)
	}()

	if c.createAgencySocket() != nil {
		close(sigs)
		os.Exit(1)
	}

	if c.sendBets() != nil {
		close(sigs)
		if c.config.BetReader != nil {
			c.config.BetReader.Close()
		}
		os.Exit(1)
	}

	c.conn.Close()
}

func (c *Agency) sendBets() error {
	c.config.BetReader = NewBetReader(c.config.ID, c.config.MaxBatchAmount)
	if c.config.BetReader == nil {
		return errors.New("error creating bet reader")
	}

	batchNumber := 0
	for {
		batch, err := c.config.BetReader.ReadNextBatch()
		if err != nil {
			log.Criticalf(
				"action: read_file | result: fail | agency_id: %v | error: %v",
				c.config.ID,
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
		sendError := c.sendBatch(length, batch)
		c.conn.Read(make([]byte, RESPONSE_SIZE))
		if sendError != nil {
			log.Criticalf(`action: apuestas_enviadas | result: fail | bytes: %v | batch: %v | error: %v`, length, batchNumber, err)
			return sendError
		}

		log.Infof(`action: apuestas_enviadas | result: success | bytes: %v | batch: %v`, length, batchNumber)
	}

	c.config.BetReader.Close()
	c.config.BetReader = nil

	return nil
}

func addBatchBytesLen(batch []byte) []byte {
	batchSize := uint16(len(batch))
	batchWithLength := []byte{
		byte(batchSize >> 8),
		byte(batchSize & 0xff),
	}

	return append(batchWithLength, batch...)
}

func (c *Agency) sendBatch(length int, batch []byte) error {
	for length > 0 {
		n, err := c.conn.Write(batch)
		if errors.Is(err, io.ErrClosedPipe) {
			return err
		}

		batch = batch[n:]
		length -= n
	}

	return nil
}

func (c *Agency) HandleSIGTERM(sigs chan os.Signal) {
	if c.conn != nil {
		err := c.conn.Close()
		if err == nil {
			log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
		}
	}

	if c.config.BetReader != nil {
		c.config.BetReader.Close()
	}

	if sigs != nil {
		close(sigs)
		log.Infof("action: close_client | result: success | client_id: %v", c.config.ID)
	}
}
