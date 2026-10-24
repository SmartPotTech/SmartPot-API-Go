// Package mongodb crea el cliente de MongoDB. La conexión es perezosa: la API
// arranca aunque la base no responda todavía y /health informa el estado real.
package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

// Client envuelve el cliente de Mongo y la base de datos por defecto de la URI.
type Client struct {
	Mongo *mongo.Client
	DB    *mongo.Database
}

// Connect crea el cliente a partir de la URI (misma que usa la versión en Java).
func Connect(ctx context.Context, uri string) (*Client, error) {
	m, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear el cliente de Mongo: %w", err)
	}
	return &Client{Mongo: m, DB: m.Database(databaseName(uri))}, nil
}

// Ping comprueba que la base responde.
func (c *Client) Ping(ctx context.Context) error {
	return c.Mongo.Ping(ctx, readpref.Primary())
}

// Disconnect cierra la conexión.
func (c *Client) Disconnect(ctx context.Context) error {
	return c.Mongo.Disconnect(ctx)
}

func databaseName(uri string) string {
	if cs, err := connstring.ParseAndValidate(uri); err == nil && cs.Database != "" {
		return cs.Database
	}
	return "smartpot"
}
