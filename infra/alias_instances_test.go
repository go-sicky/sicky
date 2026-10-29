/**
 * @file alias_instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import "testing"

// The deprecated Clickhouse/Nats spellings must forward names correctly rather
// than silently dropping the variadic argument.
func TestDeprecatedInstanceAliases(t *testing.T) {
	ClearClickHouseAll()
	ClearNATSAll()
	t.Cleanup(func() {
		ClearClickHouseAll()
		ClearNATSAll()
	})

	if GetClickhouse("missing") != nil {
		t.Fatal("GetClickhouse(missing) = non-nil")
	}

	if GetNats("missing") != nil {
		t.Fatal("GetNats(missing) = non-nil")
	}

	if db, err := InitClickhouse(nil); err != nil || db != nil {
		t.Fatalf("InitClickhouse(nil) = (%v, %v), want (nil, nil)", db, err)
	}

	if conn, err := InitNats(nil); err != nil || conn != nil {
		t.Fatalf("InitNats(nil) = (%v, %v), want (nil, nil)", conn, err)
	}

	ClearClickhouse("missing")
	ClearNats("missing")
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
