package person

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS persons (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(255) NOT NULL,
    age     INTEGER,
    address VARCHAR(255),
    work    VARCHAR(255)
)`

// PgxRepository — реализация хранилища на PostgreSQL (pgx).
type PgxRepository struct {
	pool *pgxpool.Pool
}

// NewPgxRepository подключается к БД и при необходимости создаёт схему.
func NewPgxRepository(ctx context.Context, dsn string) (*PgxRepository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &PgxRepository{pool: pool}, nil
}

func (r *PgxRepository) Close() { r.pool.Close() }

func (r *PgxRepository) Create(ctx context.Context, p Person) (int32, error) {
	var id int32
	err := r.pool.QueryRow(ctx,
		`INSERT INTO persons (name, age, address, work) VALUES ($1, $2, $3, $4) RETURNING id`,
		p.Name, p.Age, p.Address, p.Work,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert person: %w", err)
	}
	return id, nil
}

func (r *PgxRepository) GetByID(ctx context.Context, id int32) (Person, error) {
	var p Person
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, age, address, work FROM persons WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Age, &p.Address, &p.Work)
	if errors.Is(err, pgx.ErrNoRows) {
		return Person{}, ErrNotFound
	}
	if err != nil {
		return Person{}, fmt.Errorf("select person: %w", err)
	}
	return p, nil
}

func (r *PgxRepository) List(ctx context.Context) ([]Person, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, age, address, work FROM persons ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("select persons: %w", err)
	}
	defer rows.Close()

	var people []Person
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name, &p.Age, &p.Address, &p.Work); err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}
		people = append(people, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return people, nil
}

func (r *PgxRepository) Update(ctx context.Context, id int32, p Person) (Person, error) {
	var saved Person
	err := r.pool.QueryRow(ctx,
		`UPDATE persons SET name = $1, age = $2, address = $3, work = $4 WHERE id = $5
         RETURNING id, name, age, address, work`,
		p.Name, p.Age, p.Address, p.Work, id,
	).Scan(&saved.ID, &saved.Name, &saved.Age, &saved.Address, &saved.Work)
	if errors.Is(err, pgx.ErrNoRows) {
		return Person{}, ErrNotFound
	}
	if err != nil {
		return Person{}, fmt.Errorf("update person: %w", err)
	}
	return saved, nil
}

func (r *PgxRepository) Delete(ctx context.Context, id int32) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM persons WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete person: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
