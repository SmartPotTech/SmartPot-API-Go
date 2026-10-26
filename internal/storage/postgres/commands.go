package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/actuators"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/commands"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CommandRepository implementa commands.Repository sobre PostgreSQL (tabla commands).
type CommandRepository struct {
	pool *pgxpool.Pool
}

// NewCommandRepository crea el adaptador de órdenes.
func NewCommandRepository(pool *pgxpool.Pool) *CommandRepository {
	return &CommandRepository{pool: pool}
}

const commandColumns = "id, crop_id, actuator_id, actuator_type, action, duration_seconds, status, source, reason, message, created_at, sent_at, completed_at"

func scanCommand(row pgx.Row) (*commands.Command, error) {
	var (
		c                                 commands.Command
		id, cropID, actuatorID            string
		actuatorType, action, status, src string
	)
	if err := row.Scan(&id, &cropID, &actuatorID, &actuatorType, &action, &c.DurationSeconds,
		&status, &src, &c.Reason, &c.Message, &c.CreatedAt, &c.SentAt, &c.CompletedAt); err != nil {
		return nil, err
	}
	var err error
	if c.ID, err = parseOID(id); err != nil {
		return nil, err
	}
	if c.CropID, err = parseOID(cropID); err != nil {
		return nil, err
	}
	if c.ActuatorID, err = parseOID(actuatorID); err != nil {
		return nil, err
	}
	c.ActuatorType = actuators.Type(actuatorType)
	c.Action = commands.Action(action)
	c.Status = commands.Status(status)
	c.Source = commands.Source(src)
	return &c, nil
}

func (r *CommandRepository) query(ctx context.Context, sql string, args ...any) ([]commands.Command, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]commands.Command, 0)
	for rows.Next() {
		c, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// Insert guarda una orden nueva y fija su id generado.
func (r *CommandRepository) Insert(ctx context.Context, c *commands.Command) error {
	if c.ID.IsZero() {
		c.ID = primitive.NewObjectID()
	}
	_, err := r.pool.Exec(ctx,
		"INSERT INTO commands ("+commandColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)",
		hexOf(c.ID), hexOf(c.CropID), hexOf(c.ActuatorID), string(c.ActuatorType), string(c.Action),
		c.DurationSeconds, string(c.Status), string(c.Source), c.Reason, c.Message, c.CreatedAt, c.SentAt, c.CompletedAt)
	return err
}

// Save reemplaza la orden.
func (r *CommandRepository) Save(ctx context.Context, c *commands.Command) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE commands SET crop_id=$2, actuator_id=$3, actuator_type=$4, action=$5, duration_seconds=$6, status=$7, source=$8, reason=$9, message=$10, created_at=$11, sent_at=$12, completed_at=$13 WHERE id=$1",
		hexOf(c.ID), hexOf(c.CropID), hexOf(c.ActuatorID), string(c.ActuatorType), string(c.Action),
		c.DurationSeconds, string(c.Status), string(c.Source), c.Reason, c.Message, c.CreatedAt, c.SentAt, c.CompletedAt)
	return err
}

// ListByCrop devuelve el historial del cultivo, más recientes primero.
func (r *CommandRepository) ListByCrop(ctx context.Context, cropID primitive.ObjectID, limit int) ([]commands.Command, error) {
	return r.query(ctx,
		"SELECT "+commandColumns+" FROM commands WHERE crop_id = $1 ORDER BY created_at DESC LIMIT $2", hexOf(cropID), limit)
}

// ListByCrops devuelve el historial de varios cultivos, más recientes primero.
func (r *CommandRepository) ListByCrops(ctx context.Context, cropIDs []primitive.ObjectID, limit int) ([]commands.Command, error) {
	if len(cropIDs) == 0 {
		return nil, nil
	}
	return r.query(ctx,
		"SELECT "+commandColumns+" FROM commands WHERE crop_id = ANY($1) ORDER BY created_at DESC LIMIT $2", hexSlice(cropIDs), limit)
}

// CountByCropsSince cuenta los comandos de varios cultivos desde un instante.
func (r *CommandRepository) CountByCropsSince(ctx context.Context, cropIDs []primitive.ObjectID, since time.Time) (int64, error) {
	if len(cropIDs) == 0 {
		return 0, nil
	}
	var n int64
	err := r.pool.QueryRow(ctx,
		"SELECT count(*) FROM commands WHERE crop_id = ANY($1) AND created_at > $2", hexSlice(cropIDs), since).Scan(&n)
	return n, err
}

// FindByIDAndCrop busca un comando del cultivo.
func (r *CommandRepository) FindByIDAndCrop(ctx context.Context, id, cropID primitive.ObjectID) (*commands.Command, error) {
	c, err := scanCommand(r.pool.QueryRow(ctx,
		"SELECT "+commandColumns+" FROM commands WHERE id = $1 AND crop_id = $2", hexOf(id), hexOf(cropID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

// FindStaleSent devuelve los comandos SENT enviados antes del límite.
func (r *CommandRepository) FindStaleSent(ctx context.Context, before time.Time) ([]commands.Command, error) {
	return r.query(ctx,
		"SELECT "+commandColumns+" FROM commands WHERE status = 'SENT' AND sent_at < $1", before)
}

// ExistsInFlight indica si el actuador tiene un comando PENDING o SENT.
func (r *CommandRepository) ExistsInFlight(ctx context.Context, actuatorID primitive.ObjectID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM commands WHERE actuator_id = $1 AND status IN ('PENDING','SENT'))", hexOf(actuatorID)).Scan(&exists)
	return exists, err
}

// DeleteByCrop borra los comandos de un cultivo.
func (r *CommandRepository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM commands WHERE crop_id = $1", hexOf(cropID))
	return err
}

// DeleteOlderThan borra las órdenes anteriores al corte.
func (r *CommandRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, "DELETE FROM commands WHERE created_at < $1", cutoff)
	return tag.RowsAffected(), err
}
