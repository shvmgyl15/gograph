package features

import (
	"database/sql"
	"errors"
	"fmt"
)

type BaseUser struct {
	ID int
}

type AdminUser struct {
	BaseUser
	Role string
}

func FetchUser(db *sql.DB, id int) (*AdminUser, error) {
	if db == nil {
		panic("database is nil")
	}

	row := db.QueryRow("SELECT id, role FROM admins WHERE id = ?", id)
	var admin AdminUser
	if err := row.Scan(&admin.ID, &admin.Role); err != nil {
		return nil, fmt.Errorf("failed to scan admin: %w", err)
	}

	if admin.Role == "" {
		return nil, errors.New("admin role is empty")
	}

	return &admin, nil
}
