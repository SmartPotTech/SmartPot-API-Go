package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/users"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserRepository implementa users.Repository sobre PostgreSQL (tabla users).
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository crea el adaptador de cuentas.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const userColumns = "id, name, last_name, email, password_hash, role, created_at, updated_at"

func scanUser(row pgx.Row) (*users.User, error) {
	var (
		u         users.User
		id, role  string
		updatedAt *time.Time
	)
	if err := row.Scan(&id, &u.Name, &u.LastName, &u.Email, &u.PasswordHash, &role, &u.CreatedAt, &updatedAt); err != nil {
		return nil, err
	}
	oid, err := parseOID(id)
	if err != nil {
		return nil, err
	}
	u.ID = oid
	u.Role = users.Role(role)
	if updatedAt != nil {
		u.UpdatedAt = *updatedAt
	}
	return &u, nil
}

// FindByEmail devuelve el usuario con ese correo, o nil si no existe.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*users.User, error) {
	row := r.pool.QueryRow(ctx, "SELECT "+userColumns+" FROM users WHERE email = $1", email)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// ExistsByEmail indica si ya hay una cuenta con ese correo.
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	return exists, err
}

// FindByID devuelve el usuario por id, o nil si el id es inválido o no existe.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*users.User, error) {
	oid, err := parseOID(id)
	if err != nil {
		return nil, nil
	}
	row := r.pool.QueryRow(ctx, "SELECT "+userColumns+" FROM users WHERE id = $1", oid.Hex())
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// Insert guarda un usuario nuevo y fija su id generado. Un correo duplicado devuelve users.ErrDuplicateEmail.
func (r *UserRepository) Insert(ctx context.Context, u *users.User) error {
	if u.ID.IsZero() {
		u.ID = primitive.NewObjectID()
	}
	_, err := r.pool.Exec(ctx,
		"INSERT INTO users ("+userColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8)",
		hexOf(u.ID), u.Name, u.LastName, u.Email, u.PasswordHash, string(u.Role), u.CreatedAt, nullTime(u.UpdatedAt))
	if isUniqueViolation(err) {
		return users.ErrDuplicateEmail
	}
	return err
}

// Update reemplaza los datos del usuario.
func (r *UserRepository) Update(ctx context.Context, u *users.User) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE users SET name=$2, last_name=$3, email=$4, password_hash=$5, role=$6, created_at=$7, updated_at=$8 WHERE id=$1",
		hexOf(u.ID), u.Name, u.LastName, u.Email, u.PasswordHash, string(u.Role), u.CreatedAt, nullTime(u.UpdatedAt))
	if isUniqueViolation(err) {
		return users.ErrDuplicateEmail
	}
	return err
}

// Delete borra el usuario por id.
func (r *UserRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", hexOf(id))
	return err
}
