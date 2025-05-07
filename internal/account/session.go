package account

import (
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"

	"github.com/gotd/td/crypto"
	"github.com/gotd/td/session"
	"github.com/pkg/errors"
)

const latestTelethonVersion byte = '1'

func StringSession(hx string) (*session.Data, error) {
	if len(hx) < 1 {
		return nil, errors.Errorf("given string too small: %d", len(hx))
	}
	version := hx[0]
	if version != latestTelethonVersion {
		return nil, errors.Errorf("unexpected version %q, latest supported is %q",
			version,
			latestTelethonVersion,
		)
	}

	rawData, err := base64.URLEncoding.DecodeString(hx[1:])
	if err != nil {
		return nil, errors.Wrap(err, "decode hex")
	}

	data, err := decodeStringSession(rawData)
	if err != nil {
		return nil, errors.Wrap(err, "decode string session")
	}
	return data, nil

}

func decodeStringSession(data []byte) (*session.Data, error) {
	// Given parameter should contain version + data
	// where data encoded using pack as '>B4sH256s' or '>B16sH256s'
	// depending on IP type.
	//
	// Table:
	//
	// | Size |  Type  | Description |
	// |------|--------|-------------|
	// | 1    | byte   | DC ID       |
	// | 4/16 | bytes  | IP address  |
	// | 2    | uint16 | Port        |
	// | 256  | bytes  | Auth key    |
	var ipLength int
	switch len(data) {
	case 263:
		ipLength = 4
	case 275:
		ipLength = 16
	default:
		return nil, errors.Errorf("decoded hex has invalid length: %d", len(data))
	}

	// | 1    | byte   | DC ID       |
	dcID := data[0]

	// | 4/16 | bytes  | IP address  |
	addr := make(net.IP, 0, 16)
	addr = append(addr, data[1:1+ipLength]...)

	// | 2    | uint16 | Port        |
	port := binary.BigEndian.Uint16(data[1+ipLength : 3+ipLength])

	// | 256  | bytes  | Auth key    |
	var key crypto.Key
	copy(key[:], data[3+ipLength:])
	id := key.WithID().ID

	return &session.Data{
		DC:        int(dcID),
		Addr:      net.JoinHostPort(addr.String(), strconv.Itoa(int(port))),
		AuthKey:   key[:],
		AuthKeyID: id[:],
	}, nil
}

func SqiteSession(sessionPath string) (*session.Data, error) {
	db, err := sql.Open("sqlite", sessionPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var (
		dcID    int
		authKey []byte
		server  string
		port    int
	)

	err = db.QueryRow(`SELECT dc_id, auth_key, server_address, port FROM sessions LIMIT 1`).
		Scan(&dcID, &authKey, &server, &port)
	if err != nil {
		return nil, err
	}
	if len(authKey) != 256 {
		return nil, fmt.Errorf("invalid auth key length: %d", len(authKey))
	}

	ip := net.ParseIP(server)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP: %s", server)
	}

	addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))

	var key crypto.Key
	copy(key[:], authKey)

	keyWithID := key.WithID()
	id := keyWithID.ID

	return &session.Data{
		DC:        dcID,
		Addr:      addr,
		AuthKey:   key[:],
		AuthKeyID: id[:],
	}, nil
}
