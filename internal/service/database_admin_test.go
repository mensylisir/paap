package service

import (
	"strings"
	"testing"

	"paap/internal/k8s"
)

func TestDatabaseDSNBuildsMySQLAndPostgres(t *testing.T) {
	mysql := databaseDSN(k8s.DatabaseConnectionInfo{
		Driver:   "mysql",
		Host:     "mysql.ns.svc.cluster.local",
		Port:     3306,
		Username: "root",
		Password: "secret",
	})
	if !strings.Contains(mysql, "root:secret@tcp(mysql.ns.svc.cluster.local:3306)/") || !strings.Contains(mysql, "parseTime=true") {
		t.Fatalf("unexpected mysql dsn %q", mysql)
	}

	postgres := databaseDSN(k8s.DatabaseConnectionInfo{
		Driver:   "pgx",
		Host:     "postgres.ns.svc.cluster.local",
		Port:     5432,
		Username: "postgres",
		Password: "secret",
		Database: "postgres",
	})
	if postgres != "postgres://postgres:secret@postgres.ns.svc.cluster.local:5432/postgres?sslmode=disable" {
		t.Fatalf("unexpected postgres dsn %q", postgres)
	}
}

func TestDatabaseTableQueryUsesTargetDatabase(t *testing.T) {
	info := k8s.DatabaseConnectionInfo{Driver: "pgx", Database: "postgres"}
	got := databaseInfoForTarget(info, "appdb")
	if got.Database != "appdb" {
		t.Fatalf("expected target database, got %#v", got)
	}
}

func TestQuoteTableNameEscapesIdentifiers(t *testing.T) {
	if got := quoteTableName("mysql", "we`ird"); got != "`we``ird`" {
		t.Fatalf("unexpected mysql quote %q", got)
	}
	if got := quoteTableName("pgx", `public.we"ird`); got != `"public"."we""ird"` {
		t.Fatalf("unexpected postgres quote %q", got)
	}
}

func TestDatabaseCRUDSQLUsesQuotedIdentifiersAndPlaceholders(t *testing.T) {
	info := k8s.DatabaseConnectionInfo{Driver: "mysql", Database: "appdb"}

	query, args, err := insertRowSQL(info, "orders", map[string]string{"name": "demo", "status": "new"})
	if err != nil {
		t.Fatalf("insert sql: %v", err)
	}
	if query != "INSERT INTO `orders` (`name`, `status`) VALUES (?, ?)" {
		t.Fatalf("unexpected insert query %q", query)
	}
	if len(args) != 2 || args[0] != "demo" || args[1] != "new" {
		t.Fatalf("unexpected insert args %#v", args)
	}

	query, args, err = updateRowSQL(info, "orders", map[string]string{"status": "done"}, map[string]string{"id": "42"})
	if err != nil {
		t.Fatalf("update sql: %v", err)
	}
	if query != "UPDATE `orders` SET `status` = ? WHERE `id` = ?" {
		t.Fatalf("unexpected update query %q", query)
	}
	if len(args) != 2 || args[0] != "done" || args[1] != "42" {
		t.Fatalf("unexpected update args %#v", args)
	}

	query, args, err = deleteRowSQL(info, "orders", map[string]string{"id": "42"})
	if err != nil {
		t.Fatalf("delete sql: %v", err)
	}
	if query != "DELETE FROM `orders` WHERE `id` = ?" {
		t.Fatalf("unexpected delete query %q", query)
	}
	if len(args) != 1 || args[0] != "42" {
		t.Fatalf("unexpected delete args %#v", args)
	}
}

func TestParseSQLObjectRequiresJSONMap(t *testing.T) {
	got, err := ParseSQLObject(`{"id":"42","name":"demo"}`)
	if err != nil {
		t.Fatalf("parse object: %v", err)
	}
	if got["id"] != "42" || got["name"] != "demo" {
		t.Fatalf("unexpected parsed object %#v", got)
	}

	if _, err := ParseSQLObject(`["bad"]`); err == nil {
		t.Fatalf("expected non-object json to fail")
	}
}

func TestParseTableColumnsBuildsDriverSpecificColumns(t *testing.T) {
	columns, err := ParseTableColumns("id:serial primary key\nname:varchar(120)\nactive:boolean not null")
	if err != nil {
		t.Fatalf("parse columns: %v", err)
	}
	if len(columns) != 3 || columns[0].Name != "id" || columns[1].DataType != "varchar(120)" {
		t.Fatalf("unexpected columns %#v", columns)
	}

	query, err := createTableSQL(k8s.DatabaseConnectionInfo{Driver: "pgx"}, "public.accounts", columns)
	if err != nil {
		t.Fatalf("create table sql: %v", err)
	}
	want := `CREATE TABLE "public"."accounts" ("id" serial primary key, "name" varchar(120), "active" boolean not null)`
	if query != want {
		t.Fatalf("unexpected create table query %q", query)
	}
}
