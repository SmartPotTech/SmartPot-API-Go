package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/notifications"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationRepository implementa notifications.Repository sobre PostgreSQL (tabla notifications).
type NotificationRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationRepository crea el adaptador de notificaciones.
func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

const notificationColumns = "id, user_id, crop_id, type, title, message, read, created_at"

func scanNotification(row pgx.Row) (*notifications.Notification, error) {
	var (
		n          notifications.Notification
		id, userID string
		cropID     *string
		typ        string
	)
	if err := row.Scan(&id, &userID, &cropID, &typ, &n.Title, &n.Message, &n.Read, &n.CreatedAt); err != nil {
		return nil, err
	}
	var err error
	if n.ID, err = parseOID(id); err != nil {
		return nil, err
	}
	if n.UserID, err = parseOID(userID); err != nil {
		return nil, err
	}
	if n.CropID, err = parseOIDPtr(cropID); err != nil {
		return nil, err
	}
	n.Type = notifications.Type(typ)
	return &n, nil
}

func (r *NotificationRepository) query(ctx context.Context, sql string, args ...any) ([]notifications.Notification, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]notifications.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, rows.Err()
}

// Insert guarda una notificación y fija su id generado.
func (r *NotificationRepository) Insert(ctx context.Context, n *notifications.Notification) error {
	if n.ID.IsZero() {
		n.ID = primitive.NewObjectID()
	}
	_, err := r.pool.Exec(ctx,
		"INSERT INTO notifications ("+notificationColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8)",
		hexOf(n.ID), hexOf(n.UserID), hexPtr(n.CropID), string(n.Type), n.Title, n.Message, n.Read, n.CreatedAt)
	return err
}

// List devuelve las notificaciones del usuario, más recientes primero.
func (r *NotificationRepository) List(ctx context.Context, userID primitive.ObjectID, unreadOnly bool, limit int) ([]notifications.Notification, error) {
	if unreadOnly {
		return r.query(ctx,
			"SELECT "+notificationColumns+" FROM notifications WHERE user_id = $1 AND read = false ORDER BY created_at DESC LIMIT $2",
			hexOf(userID), limit)
	}
	return r.query(ctx,
		"SELECT "+notificationColumns+" FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2",
		hexOf(userID), limit)
}

// CountUnread cuenta las notificaciones sin leer del usuario.
func (r *NotificationRepository) CountUnread(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, "SELECT count(*) FROM notifications WHERE user_id = $1 AND read = false", hexOf(userID)).Scan(&n)
	return n, err
}

// FindByIDAndUser busca una notificación del usuario.
func (r *NotificationRepository) FindByIDAndUser(ctx context.Context, id, userID primitive.ObjectID) (*notifications.Notification, error) {
	n, err := scanNotification(r.pool.QueryRow(ctx,
		"SELECT "+notificationColumns+" FROM notifications WHERE id = $1 AND user_id = $2", hexOf(id), hexOf(userID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return n, err
}

// SetRead marca una notificación como leída.
func (r *NotificationRepository) SetRead(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "UPDATE notifications SET read = true WHERE id = $1", hexOf(id))
	return err
}

// MarkAllRead marca como leídas todas las del usuario.
func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "UPDATE notifications SET read = true WHERE user_id = $1 AND read = false", hexOf(userID))
	return err
}

// DeleteByIDAndUser borra una notificación del usuario; devuelve cuántas borró.
func (r *NotificationRepository) DeleteByIDAndUser(ctx context.Context, id, userID primitive.ObjectID) (int64, error) {
	tag, err := r.pool.Exec(ctx, "DELETE FROM notifications WHERE id = $1 AND user_id = $2", hexOf(id), hexOf(userID))
	return tag.RowsAffected(), err
}

// DeleteByUser borra todas las notificaciones del usuario.
func (r *NotificationRepository) DeleteByUser(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM notifications WHERE user_id = $1", hexOf(userID))
	return err
}

// DeleteByCrop borra todas las notificaciones de un cultivo.
func (r *NotificationRepository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM notifications WHERE crop_id = $1", hexOf(cropID))
	return err
}

// FindByCropCreatedAfter devuelve las notificaciones de un cultivo posteriores a un instante, más antiguas primero.
func (r *NotificationRepository) FindByCropCreatedAfter(ctx context.Context, cropID primitive.ObjectID, after time.Time) ([]notifications.Notification, error) {
	return r.query(ctx,
		"SELECT "+notificationColumns+" FROM notifications WHERE crop_id = $1 AND created_at > $2 ORDER BY created_at ASC",
		hexOf(cropID), after)
}

// CountByCropTypeCreatedAfter cuenta por cultivo, tipo e instante.
func (r *NotificationRepository) CountByCropTypeCreatedAfter(ctx context.Context, cropID primitive.ObjectID, t notifications.Type, after time.Time) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx,
		"SELECT count(*) FROM notifications WHERE crop_id = $1 AND type = $2 AND created_at > $3",
		hexOf(cropID), string(t), after).Scan(&n)
	return n, err
}

// DeleteOlderThan borra las notificaciones anteriores al corte.
func (r *NotificationRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, "DELETE FROM notifications WHERE created_at < $1", cutoff)
	return tag.RowsAffected(), err
}
