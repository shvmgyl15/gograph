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

func MakeAdmin(role string) *AdminUser {
	return &AdminUser{
		BaseUser: BaseUser{ID: 0},
		Role:     role,
	}
}

func helper() (int, error) { return 0, nil }

func ReturnUsageExamples() {
	// discarded
	helper()
	// assigned
	n, err := helper()
	_ = n
	_ = err
	// partially_ignored
	_, err2 := helper()
	_ = err2
	// returned (in a wrapper)
	_ = func() (int, error) { return helper() }
}
