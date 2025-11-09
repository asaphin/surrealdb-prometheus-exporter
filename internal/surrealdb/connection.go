package surrealdb

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go"
)

const commonConnectionKey = "__common__"

type Config interface {
	SurrealURL() string
	SurrealUsername() string
	SurrealPassword() string
	SurrealTimeout() time.Duration // TODO figure out if required
}

type ConnectionManager interface {
	Get(ctx context.Context, ns, db string) (*surrealdb.DB, error)
}

type singleConnectionManager struct {
	conn   *surrealdb.DB
	lastNS string
	lastDB string
	mu     sync.Mutex
	cfg    Config
}

func NewSingleConnectionManager(cfg Config) *singleConnectionManager {
	return &singleConnectionManager{
		cfg: cfg,
		mu:  sync.Mutex{},
	}
}

func (m *singleConnectionManager) Get(ctx context.Context, ns, db string) (*surrealdb.DB, error) {
	if (ns == "") != (db == "") {
		return nil, fmt.Errorf("namespace and database must both be provided or both be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil {
		conn, err := createConnection(ctx, m.cfg, ns, db)
		if err != nil {
			return nil, err
		}

		m.conn = conn
	}

	if ns != "" && (ns != m.lastNS || db != m.lastDB) {
		if err := m.conn.Use(ctx, ns, db); err != nil {
			return nil, fmt.Errorf("unable to use namespace/database: %w", err)
		}

		m.lastNS = ns
		m.lastDB = db
	}

	return m.conn, nil
}

type multiConnectionManager struct {
	connections sync.Map
	creating    sync.Map
	cfg         Config
}

func NewMultiConnectionManager(cfg Config) *multiConnectionManager {
	return &multiConnectionManager{
		connections: sync.Map{},
		creating:    sync.Map{},
		cfg:         cfg,
	}
}

func (m *multiConnectionManager) Get(ctx context.Context, ns, db string) (*surrealdb.DB, error) {
	if (ns == "" && db != "") || (ns != "" && db == "") {
		return nil, fmt.Errorf("namespace and database must both be provided or both be empty")
	}

	if ns != "" { // it is not necessary to check db value after first condition
		return m.getOrCreate(ctx, ns+":"+db, ns, db)
	}

	return m.getOrCreate(ctx, commonConnectionKey, "", "")
}

func (m *multiConnectionManager) getOrCreate(ctx context.Context, key, ns, db string) (*surrealdb.DB, error) {
	if conn, ok := m.connections.Load(key); ok {
		return conn.(*surrealdb.DB), nil
	}

	mutexInterface, _ := m.creating.LoadOrStore(key, &sync.Mutex{})
	mutex := mutexInterface.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()

	if conn, ok := m.connections.Load(key); ok {
		return conn.(*surrealdb.DB), nil
	}

	newConn, err := createConnection(ctx, m.cfg, ns, db)
	if err != nil {
		return nil, err
	}

	m.connections.Store(key, newConn)

	return newConn, nil
}

func createConnection(ctx context.Context, cfg Config, ns, db string) (*surrealdb.DB, error) {
	conn, err := surrealdb.FromEndpointURLString(ctx, cfg.SurrealURL())
	if err != nil {
		return nil, fmt.Errorf("unable to connect to SurrealDB: %w", err)
	}

	authData := &surrealdb.Auth{
		Username: cfg.SurrealUsername(),
		Password: cfg.SurrealPassword(),
	}

	token, err := conn.SignIn(ctx, authData)
	if err != nil {
		closeConnectionWithWarning(ctx, conn)
		return nil, fmt.Errorf("unable to sign in to SurrealDB: %w", err)
	}

	if err = conn.Authenticate(ctx, token); err != nil {
		closeConnectionWithWarning(ctx, conn)
		return nil, fmt.Errorf("unable to authenticate: %w", err)
	}

	if ns != "" && db != "" {
		if err = conn.Use(ctx, ns, db); err != nil {
			closeConnectionWithWarning(ctx, conn)
			return nil, fmt.Errorf("unable to use namespace/database: %w", err)
		}
	}

	return conn, nil
}

func closeConnectionWithWarning(ctx context.Context, conn *surrealdb.DB) {
	err := conn.Close(ctx)
	if err != nil {
		slog.Warn("unable to close connection", "error", err)
	}
}
