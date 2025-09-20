package arangodb

import (
	"context"
	"fmt"
	"init-db/internal/config"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
)

var (
	arangoOnce sync.Once
	AInstance  *ArangoDBClient
)

type ArangoDBClient struct {
	db driver.Database
}

func GetInstance() *ArangoDBClient {
	return AInstance
}

func Init() error {
	var err error
	arangoOnce.Do(func() {
		cfg := config.AppConfig.ArangoDB
		if cfg.Addr == "" || cfg.Database == "" {
			err = fmt.Errorf("arangodb address or database not configured")
			return
		}

		conn, connErr := http.NewConnection(http.ConnectionConfig{
			Endpoints: strings.Split(cfg.Addr, ","),
		})
		if connErr != nil {
			err = fmt.Errorf("failed to create arangodb connection: %w", connErr)
			return
		}

		client, clientErr := driver.NewClient(driver.ClientConfig{
			Connection:     conn,
			Authentication: driver.BasicAuthentication(cfg.Username, cfg.Password),
		})
		if clientErr != nil {
			err = fmt.Errorf("failed to create arangodb client: %w", clientErr)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, dbErr := client.Database(ctx, cfg.Database)
		if dbErr != nil {
			err = fmt.Errorf("failed to connect to arangodb database %s: %w", cfg.Database, dbErr)
			return
		}

		AInstance = &ArangoDBClient{
			db: db,
		}
		log.Printf("ArangoDB connect successed to database: %s", cfg.Database)
	})
	return err
}

func (a *ArangoDBClient) GetDB() driver.Database {
	return a.db
}
