package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"paap/internal/k8s"
)

type DatabaseTable struct {
	Name string
	Type string
}

type DatabaseColumn struct {
	Name     string
	DataType string
	Nullable string
	Default  string
}

type TableColumnDefinition struct {
	Name     string
	DataType string
}

type DatabaseBackupDocument struct {
	Engine    string                `json:"engine"`
	Database  string                `json:"database"`
	CreatedAt string                `json:"createdAt"`
	Tables    []DatabaseBackupTable `json:"tables"`
}

type DatabaseBackupTable struct {
	Name    string              `json:"name"`
	Type    string              `json:"type"`
	Columns []DatabaseColumn    `json:"columns"`
	Rows    []map[string]string `json:"rows"`
}

type DatabaseBackupSummary struct {
	Engine     string
	Database   string
	CreatedAt  string
	TableCount int
	RowCount   int
	SizeBytes  int
}

func CheckDatabaseConnection(ctx context.Context, info k8s.DatabaseConnectionInfo) error {
	db, err := openDatabase(info)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.PingContext(ctx)
}

func ListDatabases(ctx context.Context, info k8s.DatabaseConnectionInfo) ([]string, error) {
	db, err := openDatabase(info)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := "SHOW DATABASES"
	if isPostgresDriver(info.Driver) {
		query = "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname"
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func ListDatabaseTables(ctx context.Context, info k8s.DatabaseConnectionInfo, database string) ([]DatabaseTable, error) {
	info = databaseInfoForTarget(info, database)
	db, err := openDatabase(info)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var rows *sql.Rows
	if isPostgresDriver(info.Driver) {
		rows, err = db.QueryContext(ctx, "SELECT table_schema || '.' || table_name, table_type FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog', 'information_schema') ORDER BY table_schema, table_name")
	} else {
		rows, err = db.QueryContext(ctx, "SELECT table_name, table_type FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name", info.Database)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]DatabaseTable, 0)
	for rows.Next() {
		var table DatabaseTable
		if err := rows.Scan(&table.Name, &table.Type); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func ListTableColumns(ctx context.Context, info k8s.DatabaseConnectionInfo, database, table string) ([]DatabaseColumn, error) {
	info = databaseInfoForTarget(info, database)
	db, err := openDatabase(info)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var rows *sql.Rows
	if isPostgresDriver(info.Driver) {
		schemaName, tableName := splitSchemaTable(table)
		rows, err = db.QueryContext(ctx, "SELECT column_name, data_type, is_nullable, COALESCE(column_default, '') FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position", schemaName, tableName)
	} else {
		rows, err = db.QueryContext(ctx, "SELECT column_name, data_type, is_nullable, COALESCE(column_default, '') FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position", info.Database, table)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]DatabaseColumn, 0)
	for rows.Next() {
		var column DatabaseColumn
		if err := rows.Scan(&column.Name, &column.DataType, &column.Nullable, &column.Default); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func PreviewTableRows(ctx context.Context, info k8s.DatabaseConnectionInfo, database, table string, limit int) ([]map[string]string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return queryTableRows(ctx, info, database, table, " LIMIT "+strconv.Itoa(limit))
}

func ExportDatabaseBackup(ctx context.Context, info k8s.DatabaseConnectionInfo, database string) ([]byte, DatabaseBackupSummary, error) {
	info = databaseInfoForTarget(info, database)
	if strings.TrimSpace(info.Database) == "" {
		return nil, DatabaseBackupSummary{}, fmt.Errorf("database is required")
	}
	tables, err := ListDatabaseTables(ctx, info, info.Database)
	if err != nil {
		return nil, DatabaseBackupSummary{}, err
	}
	document := DatabaseBackupDocument{
		Engine:    databaseEngineName(info),
		Database:  info.Database,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Tables:    make([]DatabaseBackupTable, 0, len(tables)),
	}
	rowCount := 0
	for _, table := range tables {
		columns, err := ListTableColumns(ctx, info, info.Database, table.Name)
		if err != nil {
			return nil, DatabaseBackupSummary{}, fmt.Errorf("list columns for %s: %w", table.Name, err)
		}
		rows, err := queryTableRows(ctx, info, info.Database, table.Name, "")
		if err != nil {
			return nil, DatabaseBackupSummary{}, fmt.Errorf("read rows for %s: %w", table.Name, err)
		}
		rowCount += len(rows)
		document.Tables = append(document.Tables, DatabaseBackupTable{
			Name:    table.Name,
			Type:    table.Type,
			Columns: columns,
			Rows:    rows,
		})
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, DatabaseBackupSummary{}, err
	}
	return data, DatabaseBackupSummary{
		Engine:     document.Engine,
		Database:   document.Database,
		CreatedAt:  document.CreatedAt,
		TableCount: len(document.Tables),
		RowCount:   rowCount,
		SizeBytes:  len(data),
	}, nil
}

func queryTableRows(ctx context.Context, info k8s.DatabaseConnectionInfo, database, table, suffix string) ([]map[string]string, error) {
	info = databaseInfoForTarget(info, database)
	db, err := openDatabase(info)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := "SELECT * FROM " + quoteTableName(info.Driver, table) + suffix
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]string, 0)
	for rows.Next() {
		raw := make([]interface{}, len(columnNames))
		dest := make([]interface{}, len(columnNames))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		item := make(map[string]string, len(columnNames))
		for i, name := range columnNames {
			item[name] = stringifySQLValue(raw[i])
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func CreateDatabase(ctx context.Context, info k8s.DatabaseConnectionInfo, name string) error {
	query, err := createDatabaseSQL(info, name)
	if err != nil {
		return err
	}
	db, err := openDatabase(info)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, query)
	return err
}

func DropDatabase(ctx context.Context, info k8s.DatabaseConnectionInfo, name string) error {
	query, err := dropDatabaseSQL(info, name)
	if err != nil {
		return err
	}
	db, err := openDatabase(info)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, query)
	return err
}

func CreateTable(ctx context.Context, info k8s.DatabaseConnectionInfo, database, table string, columns []TableColumnDefinition) error {
	info = databaseInfoForTarget(info, database)
	query, err := createTableSQL(info, table, columns)
	if err != nil {
		return err
	}
	db, err := openDatabase(info)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, query)
	return err
}

func DropTable(ctx context.Context, info k8s.DatabaseConnectionInfo, database, table string) error {
	info = databaseInfoForTarget(info, database)
	query, err := dropTableSQL(info, table)
	if err != nil {
		return err
	}
	db, err := openDatabase(info)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, query)
	return err
}

func InsertTableRow(ctx context.Context, info k8s.DatabaseConnectionInfo, database, table string, values map[string]string) error {
	info = databaseInfoForTarget(info, database)
	query, args, err := insertRowSQL(info, table, values)
	if err != nil {
		return err
	}
	db, err := openDatabase(info)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, query, args...)
	return err
}

func UpdateTableRow(ctx context.Context, info k8s.DatabaseConnectionInfo, database, table string, values, where map[string]string) error {
	info = databaseInfoForTarget(info, database)
	query, args, err := updateRowSQL(info, table, values, where)
	if err != nil {
		return err
	}
	db, err := openDatabase(info)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, query, args...)
	return err
}

func DeleteTableRow(ctx context.Context, info k8s.DatabaseConnectionInfo, database, table string, where map[string]string) error {
	info = databaseInfoForTarget(info, database)
	query, args, err := deleteRowSQL(info, table, where)
	if err != nil {
		return err
	}
	db, err := openDatabase(info)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, query, args...)
	return err
}

func ParseSQLObject(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("json object is required")
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, err
	}
	if decoded == nil {
		return nil, fmt.Errorf("json object is required")
	}
	result := make(map[string]string, len(decoded))
	for key, value := range decoded {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("empty column name is not allowed")
		}
		result[key] = stringifyJSONValue(value)
	}
	return result, nil
}

func ParseTableColumns(raw string) ([]TableColumnDefinition, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("columns are required")
	}
	lines := strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ',' })
	columns := make([]TableColumnDefinition, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, dataType, ok := strings.Cut(line, ":")
		if !ok {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return nil, fmt.Errorf("column %q must be formatted as name:type", line)
			}
			name = fields[0]
			dataType = strings.Join(fields[1:], " ")
		}
		name = strings.TrimSpace(name)
		dataType = strings.TrimSpace(dataType)
		if err := validateSQLIdentifier(name); err != nil {
			return nil, err
		}
		if err := validateColumnType(dataType); err != nil {
			return nil, err
		}
		columns = append(columns, TableColumnDefinition{Name: name, DataType: dataType})
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("at least one column is required")
	}
	return columns, nil
}

func MarshalPreviewRow(row map[string]string) string {
	data, err := json.Marshal(row)
	if err != nil {
		return fmt.Sprintf("%v", row)
	}
	return string(data)
}

func openDatabase(info k8s.DatabaseConnectionInfo) (*sql.DB, error) {
	db, err := sql.Open(info.Driver, databaseDSN(info))
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	return db, nil
}

func databaseInfoForTarget(info k8s.DatabaseConnectionInfo, database string) k8s.DatabaseConnectionInfo {
	if database != "" {
		info.Database = database
	}
	return info
}

func databaseDSN(info k8s.DatabaseConnectionInfo) string {
	if isPostgresDriver(info.Driver) {
		database := info.Database
		if database == "" {
			database = "postgres"
		}
		user := url.PathEscape(info.Username)
		password := url.PathEscape(info.Password)
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", user, password, info.Host, info.Port, url.PathEscape(database))
	}

	database := info.Database
	if database != "" {
		database = "/" + database
	} else {
		database = "/"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)%s?parseTime=true&timeout=3s&readTimeout=5s&writeTimeout=5s", info.Username, info.Password, info.Host, info.Port, database)
}

func databaseEngineName(info k8s.DatabaseConnectionInfo) string {
	if isPostgresDriver(info.Driver) {
		return "postgresql"
	}
	return "mysql"
}

func splitSchemaTable(table string) (string, string) {
	parts := strings.SplitN(table, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "public", table
}

func quoteTableName(driver, table string) string {
	if isPostgresDriver(driver) {
		schemaName, tableName := splitSchemaTable(table)
		return quoteIdentifier(driver, schemaName) + "." + quoteIdentifier(driver, tableName)
	}
	return quoteIdentifier(driver, table)
}

func createDatabaseSQL(info k8s.DatabaseConnectionInfo, name string) (string, error) {
	if err := validateSQLIdentifier(name); err != nil {
		return "", err
	}
	return "CREATE DATABASE " + quoteIdentifier(info.Driver, name), nil
}

func dropDatabaseSQL(info k8s.DatabaseConnectionInfo, name string) (string, error) {
	if err := validateSQLIdentifier(name); err != nil {
		return "", err
	}
	return "DROP DATABASE " + quoteIdentifier(info.Driver, name), nil
}

func createTableSQL(info k8s.DatabaseConnectionInfo, table string, columns []TableColumnDefinition) (string, error) {
	if err := validateTableName(table); err != nil {
		return "", err
	}
	if len(columns) == 0 {
		return "", fmt.Errorf("at least one column is required")
	}
	definitions := make([]string, 0, len(columns))
	for _, column := range columns {
		if err := validateSQLIdentifier(column.Name); err != nil {
			return "", err
		}
		if err := validateColumnType(column.DataType); err != nil {
			return "", err
		}
		definitions = append(definitions, quoteIdentifier(info.Driver, column.Name)+" "+column.DataType)
	}
	return "CREATE TABLE " + quoteTableName(info.Driver, table) + " (" + strings.Join(definitions, ", ") + ")", nil
}

func dropTableSQL(info k8s.DatabaseConnectionInfo, table string) (string, error) {
	if err := validateTableName(table); err != nil {
		return "", err
	}
	return "DROP TABLE " + quoteTableName(info.Driver, table), nil
}

func insertRowSQL(info k8s.DatabaseConnectionInfo, table string, values map[string]string) (string, []interface{}, error) {
	if err := validateTableName(table); err != nil {
		return "", nil, err
	}
	keys, err := sortedSQLKeys(values)
	if err != nil {
		return "", nil, err
	}
	columns := make([]string, 0, len(keys))
	placeholders := make([]string, 0, len(keys))
	args := make([]interface{}, 0, len(keys))
	for i, key := range keys {
		columns = append(columns, quoteIdentifier(info.Driver, key))
		placeholders = append(placeholders, sqlPlaceholder(info.Driver, i+1))
		args = append(args, values[key])
	}
	query := "INSERT INTO " + quoteTableName(info.Driver, table) + " (" + strings.Join(columns, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	return query, args, nil
}

func updateRowSQL(info k8s.DatabaseConnectionInfo, table string, values, where map[string]string) (string, []interface{}, error) {
	if err := validateTableName(table); err != nil {
		return "", nil, err
	}
	valueKeys, err := sortedSQLKeys(values)
	if err != nil {
		return "", nil, fmt.Errorf("update values: %w", err)
	}
	whereKeys, err := sortedSQLKeys(where)
	if err != nil {
		return "", nil, fmt.Errorf("where values: %w", err)
	}
	assignments := make([]string, 0, len(valueKeys))
	clauses := make([]string, 0, len(whereKeys))
	args := make([]interface{}, 0, len(valueKeys)+len(whereKeys))
	index := 1
	for _, key := range valueKeys {
		assignments = append(assignments, quoteIdentifier(info.Driver, key)+" = "+sqlPlaceholder(info.Driver, index))
		args = append(args, values[key])
		index++
	}
	for _, key := range whereKeys {
		clauses = append(clauses, quoteIdentifier(info.Driver, key)+" = "+sqlPlaceholder(info.Driver, index))
		args = append(args, where[key])
		index++
	}
	query := "UPDATE " + quoteTableName(info.Driver, table) + " SET " + strings.Join(assignments, ", ") + " WHERE " + strings.Join(clauses, " AND ")
	return query, args, nil
}

func deleteRowSQL(info k8s.DatabaseConnectionInfo, table string, where map[string]string) (string, []interface{}, error) {
	if err := validateTableName(table); err != nil {
		return "", nil, err
	}
	whereKeys, err := sortedSQLKeys(where)
	if err != nil {
		return "", nil, err
	}
	clauses := make([]string, 0, len(whereKeys))
	args := make([]interface{}, 0, len(whereKeys))
	for i, key := range whereKeys {
		clauses = append(clauses, quoteIdentifier(info.Driver, key)+" = "+sqlPlaceholder(info.Driver, i+1))
		args = append(args, where[key])
	}
	query := "DELETE FROM " + quoteTableName(info.Driver, table) + " WHERE " + strings.Join(clauses, " AND ")
	return query, args, nil
}

func sortedSQLKeys(values map[string]string) ([]string, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("at least one column is required")
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if err := validateSQLIdentifier(key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func sqlPlaceholder(driver string, index int) string {
	if isPostgresDriver(driver) {
		return "$" + strconv.Itoa(index)
	}
	return "?"
}

func validateTableName(table string) error {
	table = strings.TrimSpace(table)
	if table == "" {
		return fmt.Errorf("table name is required")
	}
	if strings.Contains(table, ".") {
		schema, name := splitSchemaTable(table)
		if err := validateSQLIdentifier(schema); err != nil {
			return err
		}
		return validateSQLIdentifier(name)
	}
	return validateSQLIdentifier(table)
}

func validateSQLIdentifier(identifier string) error {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return fmt.Errorf("identifier is required")
	}
	for _, r := range identifier {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("invalid identifier %q", identifier)
	}
	return nil
}

func validateColumnType(dataType string) error {
	dataType = strings.TrimSpace(dataType)
	if dataType == "" {
		return fmt.Errorf("column type is required")
	}
	for _, blocked := range []string{";", "--", "/*", "*/"} {
		if strings.Contains(dataType, blocked) {
			return fmt.Errorf("column type contains unsupported token %q", blocked)
		}
	}
	return nil
}

func stringifyJSONValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(data)
	}
}

func quoteIdentifier(driver, identifier string) string {
	if isPostgresDriver(driver) {
		return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
	}
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func stringifySQLValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func isPostgresDriver(driver string) bool {
	return driver == "pgx" || driver == "postgres"
}
