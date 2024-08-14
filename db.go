package main

import (
	"xorm.io/xorm"

	_ "github.com/mattn/go-sqlite3"
)

// TCPClient represents a TCP client with the following properties:
// - ID: the unique identifier of the client
// - ClientID: the client ID which must be unique
// - Port: the port used by the client
type TCPClient struct {
	ID       uint16 `xorm:"id pk autoincr"`
	ClientID string `xorm:"clientId unique"`
	Port     uint16 `xorm:"port"`
}

type TcpClientRepository struct {
	engine *xorm.Engine
}

// NewTCPClientRepository creates a new TCP client repository that is responsible for managing
// TCP client data in the database. It initializes a new database engine using the sqlite3
// driver and creates a table for the TCPClient model. The function returns a pointer to the
// TcpClientRepository and an error if any occurred during the initialization process.
func NewTCPClientRepository() (*TcpClientRepository, error) {
	engine, err := xorm.NewEngine(DbDriver, DbName)
	if err != nil {
		return nil, err
	}

	err = engine.Sync(new(TCPClient))
	if err != nil {
		return nil, err
	}

	return &TcpClientRepository{engine: engine}, nil
}

// Create inserts a TCPClient into the repository.
// It takes a TCPClient object and inserts it into the database using the XORM engine.
// Returns an error if the insertion fails.
func (repo *TcpClientRepository) Create(client *TCPClient) error {
	_, err := repo.engine.Insert(client)
	return err
}

// GetByID retrieves a TCPClient from the repository by its ID.
// It searches for a client with the specified ID in the database using the XORM engine.
// Returns the client, a boolean indicating whether the client was found, and any error that occurred.
func (repo *TcpClientRepository) GetByID(id int) (*TCPClient, bool, error) {
	client := new(TCPClient)
	has, err := repo.engine.ID(id).Get(client)
	return client, has, err
}

// GetByClientID retrieves a TCP client from the repository by its client ID.
// It searches for a client with the specified client ID in the database and returns
// the client, a boolean indicating whether the client was found, and any error that occurred.
func (repo *TcpClientRepository) GetByClientID(clientID string) (*TCPClient, bool, error) {
	client := new(TCPClient)
	has, err := repo.engine.Where("clientId = ?", clientID).Get(client)
	return client, has, err
}

// GetByPort returns a slice of TCPClient instances matching the given port.
// It searches the repository for TCPClient instances with a matching port
// value and returns them. If no matching instances are found, it returns
// an empty slice. It also returns an error if there was a problem executing
// the query.
func (repo *TcpClientRepository) GetByPort(port int) ([]TCPClient, error) {
	var clients []TCPClient
	err := repo.engine.Where("port = ?", port).Find(&clients)
	return clients, err
}

// Update updates the specified TCPClient in the TcpClientRepository.
// It updates the TCPClient with the values from the client argument and returns an error if any.
func (repo *TcpClientRepository) Update(client *TCPClient) error {
	_, err := repo.engine.ID(client.ID).Update(client)
	return err
}

// DeleteByID deletes a TCP client from the repository by its ID.
// It uses the underlying Xorm engine to perform the deletion.
// The ID parameter specifies the ID of the client to be deleted.
// It returns an error if the deletion operation fails.
func (repo *TcpClientRepository) DeleteByID(id int) error {
	_, err := repo.engine.ID(id).Delete(&TCPClient{})
	return err
}

// GetAllPorts retrieves all the ports from the TCP clients in the repository.
// It returns a slice of uint16 containing all the ports and any error that occurred.
func (repo *TcpClientRepository) GetAllPorts() ([]uint16, error) {
	var clients []TCPClient
	err := repo.engine.Find(&clients)
	if err != nil {
		return nil, err
	}
	ports := make([]uint16, len(clients))
	for i, client := range clients {
		ports[i] = client.Port
	}
	return ports, nil
}
