package lottery

import "net"

func SendAll(conn net.Conn, data []byte) error {
	total := len(data)
	for total > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
		total -= n
	}
	return nil
}

