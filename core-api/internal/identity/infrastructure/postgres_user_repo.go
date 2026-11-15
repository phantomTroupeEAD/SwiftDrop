package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// PostgresUserRepository implements application.UserRepository using standard SQL database connection.
type PostgresUserRepository struct {
	db *sql.DB
}

// NewPostgresUserRepository constructs a repository backed by PostgreSQL.
func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

// FindByEmail locates a user by their case-insensitive email.
func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, display_name, phone, role, tenant_id, status, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(email))

	var (
		id, rawEmail, pwdHash, name string
		phone, tenantID             sql.NullString
		rawRole, rawStatus          string
		createdAt, updatedAt        time.Time
	)

	err := row.Scan(
		&id,
		&rawEmail,
		&pwdHash,
		&name,
		&phone,
		&rawRole,
		&tenantID,
		&rawStatus,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres query failed: %w", err)
	}

	emailVO, err := domain.NewEmail(rawEmail)
	if err != nil {
		return nil, domain.ErrInvalidEmail
	}

	var tenantPtr *string
	if tenantID.Valid {
		tenantPtr = &tenantID.String
	}

	phoneStr := ""
	if phone.Valid {
		phoneStr = phone.String
	}

	return domain.NewUser(
		id,
		emailVO,
		pwdHash,
		name,
		phoneStr,
		domain.Role(rawRole),
		tenantPtr,
		domain.UserStatus(rawStatus),
		createdAt,
		updatedAt,
	), nil
}

// FindByID locates a user by internal UUID.
func (r *PostgresUserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, display_name, phone, role, tenant_id, status, created_at, updated_at
		FROM users
		WHERE id = $1
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, id)

	var (
		uid, rawEmail, pwdHash, name string
		phone, tenantID              sql.NullString
		rawRole, rawStatus           string
		createdAt, updatedAt         time.Time
	)

	err := row.Scan(
		&uid,
		&rawEmail,
		&pwdHash,
		&name,
		&phone,
		&rawRole,
		&tenantID,
		&rawStatus,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres query failed: %w", err)
	}

	emailVO, err := domain.NewEmail(rawEmail)
	if err != nil {
		return nil, domain.ErrInvalidEmail
	}

	var tenantPtr *string
	if tenantID.Valid {
		tenantPtr = &tenantID.String
	}

	phoneStr := ""
	if phone.Valid {
		phoneStr = phone.String
	}

	return domain.NewUser(
		uid,
		emailVO,
		pwdHash,
		name,
		phoneStr,
		domain.Role(rawRole),
		tenantPtr,
		domain.UserStatus(rawStatus),
		createdAt,
		updatedAt,
	), nil
}

// Save inserts or updates a user aggregate.
func (r *PostgresUserRepository) Save(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, display_name, phone, role, tenant_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE
		SET email = EXCLUDED.email,
		    password_hash = EXCLUDED.password_hash,
		    display_name = EXCLUDED.display_name,
		    phone = EXCLUDED.phone,
		    role = EXCLUDED.role,
		    tenant_id = EXCLUDED.tenant_id,
		    status = EXCLUDED.status,
		    updated_at = EXCLUDED.updated_at;
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Email.String(),
		user.PasswordHash,
		user.DisplayName,
		user.Phone,
		string(user.Role),
		user.TenantID,
		string(user.Status),
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save user to postgres: %w", err)
	}
	return nil
}

// ListUsers returns paginated users with optional role, status, and search query filters.
func (r *PostgresUserRepository) ListUsers(ctx context.Context, filter application.UserFilter, limit int, offset int) ([]*domain.User, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, string(*filter.Role))
		argIdx++
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*filter.Status))
		argIdx++
	}

	if q := strings.TrimSpace(filter.Search); q != "" {
		conditions = append(conditions, fmt.Sprintf("(email ILIKE $%d OR display_name ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+q+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s;", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	listQuery := fmt.Sprintf(`
		SELECT id, email, password_hash, display_name, phone, role, tenant_id, status, created_at, updated_at
		FROM users
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d;
	`, whereClause, argIdx, argIdx+1)

	listArgs := append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var (
			uid, rawEmail, pwdHash, name string
			phone, tenantID              sql.NullString
			rawRole, rawStatus           string
			createdAt, updatedAt         time.Time
		)

		if err := rows.Scan(
			&uid,
			&rawEmail,
			&pwdHash,
			&name,
			&phone,
			&rawRole,
			&tenantID,
			&rawStatus,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, 0, err
		}

		emailVO, _ := domain.NewEmail(rawEmail)
		var tenantPtr *string
		if tenantID.Valid {
			tenantPtr = &tenantID.String
		}

		phoneStr := ""
		if phone.Valid {
			phoneStr = phone.String
		}

		users = append(users, domain.NewUser(
			uid,
			emailVO,
			pwdHash,
			name,
			phoneStr,
			domain.Role(rawRole),
			tenantPtr,
			domain.UserStatus(rawStatus),
			createdAt,
			updatedAt,
		))
	}

	return users, total, nil
}
