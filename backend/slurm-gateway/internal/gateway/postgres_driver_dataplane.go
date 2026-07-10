//go:build dataplane

package gateway

import _ "github.com/jackc/pgx/v5/stdlib"

func requirePostgresDriver() error {
	return nil
}
