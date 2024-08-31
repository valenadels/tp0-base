package lottery

import (
	"testing"
)

func TestBet_toBytes(t *testing.T) {
	b := &Bet{
		FirstName: "John",
		LastName:  "Doe",
		Document:  "123456789",
		Birthdate: "1990-01-01",
		Number:    "123",
	}

	expected := []byte{
		4, 74, 111, 104, 110, 
		3, 68, 111, 101, 
		9, 49, 50, 51, 52, 53, 54, 55, 56, 57, 
		10, 49, 57, 57, 48, 45, 48, 49, 45, 48, 49,
		3, 49, 50, 51, 
	}

	result := b.toBytes()

	for i, v := range result {
        if v != expected[i] {
            t.Errorf("toBytes()[%d] = %d; want %d", i, v, expected[i])
        }
    }
}