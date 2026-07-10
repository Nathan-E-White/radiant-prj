//go:build !dataplane

package gateway

import "fmt"

func requirePostgresDriver() error {
	return fmt.Errorf("postgres adapters require the dataplane build tag")
}
