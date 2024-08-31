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

// AgencyConfig Configuration used by the agency
type AgencyConfig struct {
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
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
// failure, error is printed in stdout/stderr and exit 1
// is returned
func (c *Agency) createAgencySocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
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
		return
	}
	c.sendBet()
	c.conn.Close()
}

func (c *Agency) sendBet() {
	bet := readBetFromEnv()
	betBytes := bet.toBytes()
	length := len(betBytes)

	for length > 0 {
		n, err := c.conn.Write(betBytes)
		if errors.Is(err, io.ErrClosedPipe){
			//TODO VALEN ver q hacer... reconectar?
			return 
		}

		betBytes = betBytes[n:]
		length -= n
	}

	log.Infof(`action: apuesta_enviada | result: success | dni: %v | numero: %v`, bet.Document, bet.Number)
}

func readBetFromEnv() *Bet {
	return &Bet{
		FirstName: os.Getenv("NOMBRE"),
		LastName: os.Getenv("APELLIDO"),
		Document: os.Getenv("DOCUMENTO"),
		Birthdate: os.Getenv("NACIMIENTO"),
		Number: os.Getenv("NUMERO"),
	}
}

func (c *Agency) HandleSIGTERM(sigs chan os.Signal) {
	if c.conn != nil {
		err := c.conn.Close()
		if err == nil {
			log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
		}
	}

	if sigs != nil {
		close(sigs)
		log.Infof("action: close_client | result: success | client_id: %v", c.config.ID)
	}
}
