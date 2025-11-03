package client

import (
	"context"
	"fmt"
	"time"
)

type Client interface {
	Info(ctx context.Context) (*ServerInfo, error)
	Query(ctx context.Context, query string) (interface{}, error)
	Close() error
}

type ServerInfo struct {
	Version   string
	Uptime    time.Duration
	Namespace string
	Database  string
}

type surrealDBClient struct {
	uri       string
	username  string
	password  string
	namespace string
	database  string
	timeout   time.Duration
}

func New(uri, username, password, namespace, database string, timeout time.Duration) (Client, error) {
	client := &surrealDBClient{
		uri:       uri,
		username:  username,
		password:  password,
		namespace: namespace,
		database:  database,
		timeout:   timeout,
	}

	return client, nil
}

func (c *surrealDBClient) Info(ctx context.Context) (*ServerInfo, error) {
	return &ServerInfo{
		Version:   "1.0.0-beta.9",
		Uptime:    24 * time.Hour,
		Namespace: c.namespace,
		Database:  c.database,
	}, nil
}

func (c *surrealDBClient) Query(ctx context.Context, query string) (interface{}, error) {
	return map[string]interface{}{
		"result": "ok",
	}, nil
}

func (c *surrealDBClient) Close() error {
	return nil
}

func (c *surrealDBClient) String() string {
	return fmt.Sprintf("SurrealDB[%s/%s@%s]", c.namespace, c.database, c.uri)
}
