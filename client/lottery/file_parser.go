package lottery

import (
	"encoding/csv"
	"io"
	"os"
)

const FILE_PATH = "./agency.csv"
const FIELDS_PER_RECORD = 5

type BetReader struct{
	AgencyId string
	MaxBatchAmount int
	file *os.File
	reader *csv.Reader
}

func NewBetReader(agencyId string, maxBatchAmount int) *BetReader {
	file, err := os.Open(FILE_PATH)
	if err != nil {
		log.Criticalf(
			"action: open_file | result: fail | agency_id: %v | error: %v",
			agencyId,
			err,
		)
		return nil
	}

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = FIELDS_PER_RECORD //in case of an error in the first record
	return &BetReader{
		AgencyId: agencyId,
		MaxBatchAmount: maxBatchAmount,
		file: file,
		reader: reader,
	}
}

// ReadNextBatch reads the next batch of bets from the file. 
// In case of error, nil is returned for the slice and the error is returned
func (r *BetReader) ReadNextBatch() ([]byte, error) {
	batch := make([]byte, r.MaxBatchAmount)
	eof := false
	for i := 0; i < r.MaxBatchAmount && !eof; i++ {
		record, err := r.reader.Read() 
		if err == io.EOF {
			eof = true
			break
		}

		if err != nil {
			return nil, err
		}

		batch = append(batch, CreateBetFromCsv(record, r.AgencyId).toBytes()...)
	}

	return batch, nil
}

func (r *BetReader) Close() {
	r.file.Close()
}
