package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"gopkg.in/yaml.v3"
)

type appConfig struct {
	Postgres postgresConfig
	Redis    redisConfig
	Source   string
}

type postgresConfig struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
}

type redisConfig struct {
	Host     string
	Port     int
	Password string
}

type checkResult struct {
	Name    string `json:"name"`
	Target  string `json:"target"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func main() {
	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	log.Println("orders backend listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	cfg := loadAppConfig()
	pgCheck := checkPostgres(ctx, cfg.Postgres)
	redisCheck := checkRedis(ctx, cfg.Redis)
	status := http.StatusOK
	if !pgCheck.OK || !redisCheck.OK {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"service":      "orders-backend",
		"namespace":    namespace(),
		"configSource": cfg.Source,
		"checks":       []checkResult{pgCheck, redisCheck},
		"time":         time.Now().UTC().Format(time.RFC3339),
	})
}

func loadAppConfig() appConfig {
	cfg := appConfig{
		Postgres: postgresConfig{
			Host:     env("POSTGRES_HOST", ""),
			Port:     envInt("POSTGRES_PORT", 5432),
			Database: env("POSTGRES_DATABASE", "postgres"),
			Username: env("POSTGRES_USERNAME", "postgres"),
			Password: env("POSTGRES_PASSWORD", ""),
		},
		Redis: redisConfig{
			Host:     env("REDIS_HOST", ""),
			Port:     envInt("REDIS_PORT", 6379),
			Password: env("REDIS_PASSWORD", ""),
		},
		Source: "env",
	}

	path := springConfigPath()
	if path == "" {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if applySpringConfig(&cfg, data) == nil {
		cfg.Source = path
	}
	return cfg
}

func springConfigPath() string {
	location := strings.TrimSpace(os.Getenv("SPRING_CONFIG_ADDITIONAL_LOCATION"))
	if strings.HasPrefix(location, "file:") {
		location = strings.TrimPrefix(location, "file:")
	}
	location = strings.TrimSpace(location)
	if location != "" {
		if strings.HasSuffix(location, "/") {
			return location + "application-paap.yml"
		}
		return location
	}
	if _, err := os.Stat("/etc/paap/application-paap.yml"); err == nil {
		return "/etc/paap/application-paap.yml"
	}
	return ""
}

func applySpringConfig(cfg *appConfig, data []byte) error {
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	spring := mapValue(root, "spring")
	ds := mapValue(spring, "datasource")
	if url := stringValue(ds["url"]); url != "" {
		host, port, database := parseJDBCPostgres(url)
		if host != "" {
			cfg.Postgres.Host = host
		}
		if port > 0 {
			cfg.Postgres.Port = port
		}
		if database != "" {
			cfg.Postgres.Database = database
		}
	}
	if username := stringValue(ds["username"]); username != "" {
		cfg.Postgres.Username = username
	}
	if password := resolvePlaceholder(stringValue(ds["password"])); password != "" {
		cfg.Postgres.Password = password
	}

	redisMap := mapValue(mapValue(spring, "data"), "redis")
	if host := stringValue(redisMap["host"]); host != "" {
		cfg.Redis.Host = host
	}
	if port, ok := intValue(redisMap["port"]); ok {
		cfg.Redis.Port = port
	}
	if password := resolvePlaceholder(stringValue(redisMap["password"])); password != "" {
		cfg.Redis.Password = password
	}
	return nil
}

func checkPostgres(ctx context.Context, cfg postgresConfig) checkResult {
	target := fmt.Sprintf("%s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if cfg.Host == "" {
		return checkResult{Name: "postgresql", Target: target, OK: false, Message: "missing host"}
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return checkResult{Name: "postgresql", Target: target, OK: false, Message: err.Error()}
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return checkResult{Name: "postgresql", Target: target, OK: false, Message: err.Error()}
	}
	if _, err := db.ExecContext(ctx, `create table if not exists paap_orders_check (id serial primary key, source text not null, created_at timestamptz not null default now())`); err != nil {
		return checkResult{Name: "postgresql", Target: target, OK: false, Message: err.Error()}
	}
	if _, err := db.ExecContext(ctx, `insert into paap_orders_check(source) values($1)`, "paap-real-demo"); err != nil {
		return checkResult{Name: "postgresql", Target: target, OK: false, Message: err.Error()}
	}
	var count int
	if err := db.QueryRowContext(ctx, `select count(*) from paap_orders_check`).Scan(&count); err != nil {
		return checkResult{Name: "postgresql", Target: target, OK: false, Message: err.Error()}
	}
	return checkResult{Name: "postgresql", Target: target, OK: true, Message: fmt.Sprintf("SQL read/write ok, rows=%d", count)}
}

func checkRedis(ctx context.Context, cfg redisConfig) checkResult {
	target := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if cfg.Host == "" {
		return checkResult{Name: "redis", Target: target, OK: false, Message: "missing host"}
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return checkResult{Name: "redis", Target: target, OK: false, Message: err.Error()}
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if cfg.Password != "" {
		if _, err := redisCommand(conn, reader, "AUTH", cfg.Password); err != nil {
			return checkResult{Name: "redis", Target: target, OK: false, Message: err.Error()}
		}
	}
	if _, err := redisCommand(conn, reader, "PING"); err != nil {
		return checkResult{Name: "redis", Target: target, OK: false, Message: err.Error()}
	}
	key := "paap:orders:status"
	value := time.Now().UTC().Format(time.RFC3339)
	if _, err := redisCommand(conn, reader, "SET", key, value, "EX", "300"); err != nil {
		return checkResult{Name: "redis", Target: target, OK: false, Message: err.Error()}
	}
	got, err := redisCommand(conn, reader, "GET", key)
	if err != nil {
		return checkResult{Name: "redis", Target: target, OK: false, Message: err.Error()}
	}
	if got == "" {
		return checkResult{Name: "redis", Target: target, OK: false, Message: "empty readback"}
	}
	return checkResult{Name: "redis", Target: target, OK: true, Message: "AUTH, SET and GET ok"}
}

func redisCommand(conn net.Conn, reader *bufio.Reader, args ...string) (string, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}
	if _, err := conn.Write([]byte(b.String())); err != nil {
		return "", err
	}
	return readRedisReply(reader)
}

func readRedisReply(reader *bufio.Reader) (string, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+', ':':
		return line, nil
	case '-':
		return "", fmt.Errorf("%s", line)
	case '$':
		n, err := strconv.Atoi(line)
		if err != nil {
			return "", err
		}
		if n < 0 {
			return "", nil
		}
		buf := make([]byte, n+2)
		if _, err := reader.Read(buf); err != nil {
			return "", err
		}
		return string(buf[:n]), nil
	default:
		return "", fmt.Errorf("unexpected redis reply %q%s", string(prefix), line)
	}
}

func parseJDBCPostgres(raw string) (string, int, string) {
	re := regexp.MustCompile(`^jdbc:postgresql://([^/:]+)(?::([0-9]+))?/([^?\s]+)`)
	match := re.FindStringSubmatch(strings.TrimSpace(raw))
	if len(match) == 0 {
		return "", 0, ""
	}
	port := 5432
	if match[2] != "" {
		if parsed, err := strconv.Atoi(match[2]); err == nil {
			port = parsed
		}
	}
	return match[1], port, match[3]
}

func resolvePlaceholder(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return os.Getenv(strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}"))
	}
	return value
}

func namespace() string {
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return env("POD_NAMESPACE", "")
}

func mapValue(raw interface{}, key string) map[string]interface{} {
	m, _ := raw.(map[string]interface{})
	if m == nil {
		return map[string]interface{}{}
	}
	child, _ := m[key].(map[string]interface{})
	if child == nil {
		return map[string]interface{}{}
	}
	return child
}

func stringValue(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func intValue(raw interface{}) (int, bool) {
	switch value := raw.(type) {
	case int:
		return value, true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		return parsed, err == nil
	default:
		return 0, false
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
