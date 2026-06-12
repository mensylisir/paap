package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"paap/internal/k8s"
)

type RedisSummary struct {
	Ping       string
	KeyCount   int64
	Version    string
	UsedMemory string
	Connected  string
}

type RedisKeyValue struct {
	Key   string
	Type  string
	Value string
	TTL   int64
}

func InspectRedis(ctx context.Context, info k8s.RedisConnectionInfo) (RedisSummary, error) {
	conn, reader, err := redisConnect(ctx, info)
	if err != nil {
		return RedisSummary{}, err
	}
	defer conn.Close()
	ping, err := redisCommand(conn, reader, "PING")
	if err != nil {
		return RedisSummary{}, err
	}
	dbSizeValue, err := redisCommand(conn, reader, "DBSIZE")
	if err != nil {
		return RedisSummary{}, err
	}
	infoValue, err := redisCommand(conn, reader, "INFO")
	if err != nil {
		return RedisSummary{}, err
	}
	infoMap := parseRedisInfo(fmt.Sprint(infoValue))
	return RedisSummary{
		Ping:       fmt.Sprint(ping),
		KeyCount:   redisInt(dbSizeValue),
		Version:    infoMap["redis_version"],
		UsedMemory: infoMap["used_memory_human"],
		Connected:  infoMap["connected_clients"],
	}, nil
}

func ListRedisKeys(ctx context.Context, info k8s.RedisConnectionInfo, pattern string, limit int) ([]string, error) {
	if strings.TrimSpace(pattern) == "" {
		pattern = "*"
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	conn, reader, err := redisConnect(ctx, info)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	cursor := "0"
	keys := make([]string, 0, limit)
	for {
		value, err := redisCommand(conn, reader, "SCAN", cursor, "MATCH", pattern, "COUNT", strconv.Itoa(limit))
		if err != nil {
			return nil, err
		}
		scan, ok := value.([]interface{})
		if !ok || len(scan) != 2 {
			return nil, fmt.Errorf("unexpected SCAN response %#v", value)
		}
		cursor = fmt.Sprint(scan[0])
		for _, key := range redisStringList(scan[1]) {
			keys = append(keys, key)
			if len(keys) >= limit {
				return keys, nil
			}
		}
		if cursor == "0" {
			return keys, nil
		}
	}
}

func GetRedisKey(ctx context.Context, info k8s.RedisConnectionInfo, key string) (RedisKeyValue, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return RedisKeyValue{}, fmt.Errorf("key is required")
	}
	conn, reader, err := redisConnect(ctx, info)
	if err != nil {
		return RedisKeyValue{}, err
	}
	defer conn.Close()

	keyType, err := redisCommand(conn, reader, "TYPE", key)
	if err != nil {
		return RedisKeyValue{}, err
	}
	valueType := fmt.Sprint(keyType)
	var value interface{}
	switch valueType {
	case "string":
		value, err = redisCommand(conn, reader, "GET", key)
	case "list":
		value, err = redisCommand(conn, reader, "LRANGE", key, "0", "20")
	case "set":
		value, err = redisCommand(conn, reader, "SMEMBERS", key)
	case "hash":
		value, err = redisCommand(conn, reader, "HGETALL", key)
	case "zset":
		value, err = redisCommand(conn, reader, "ZRANGE", key, "0", "20", "WITHSCORES")
	default:
		value = ""
	}
	if err != nil {
		return RedisKeyValue{}, err
	}
	ttl, err := redisCommand(conn, reader, "TTL", key)
	if err != nil {
		return RedisKeyValue{}, err
	}
	return RedisKeyValue{Key: key, Type: valueType, Value: redisValueString(value), TTL: redisInt(ttl)}, nil
}

func SetRedisKey(ctx context.Context, info k8s.RedisConnectionInfo, key, value string, ttlSeconds int) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key is required")
	}
	conn, reader, err := redisConnect(ctx, info)
	if err != nil {
		return err
	}
	defer conn.Close()
	args := []string{"SET", key, value}
	if ttlSeconds > 0 {
		args = append(args, "EX", strconv.Itoa(ttlSeconds))
	}
	_, err = redisCommand(conn, reader, args...)
	return err
}

func DeleteRedisKey(ctx context.Context, info k8s.RedisConnectionInfo, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key is required")
	}
	conn, reader, err := redisConnect(ctx, info)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = redisCommand(conn, reader, "DEL", key)
	return err
}

func ExpireRedisKey(ctx context.Context, info k8s.RedisConnectionInfo, key string, ttlSeconds int) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if ttlSeconds <= 0 {
		return fmt.Errorf("ttlSeconds must be greater than 0")
	}
	conn, reader, err := redisConnect(ctx, info)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = redisCommand(conn, reader, "EXPIRE", key, strconv.Itoa(ttlSeconds))
	return err
}

func redisConnect(ctx context.Context, info k8s.RedisConnectionInfo) (net.Conn, *bufio.Reader, error) {
	conn, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", k8s.RedisAddress(info))
	if err != nil {
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(6 * time.Second))
	reader := bufio.NewReader(conn)
	if info.Password != "" {
		if _, err := redisCommand(conn, reader, "AUTH", info.Password); err != nil {
			conn.Close()
			return nil, nil, err
		}
	}
	return conn, reader, nil
}

func redisCommand(conn net.Conn, reader *bufio.Reader, args ...string) (interface{}, error) {
	var builder strings.Builder
	builder.WriteString("*" + strconv.Itoa(len(args)) + "\r\n")
	for _, arg := range args {
		builder.WriteString("$" + strconv.Itoa(len(arg)) + "\r\n" + arg + "\r\n")
	}
	if _, err := conn.Write([]byte(builder.String())); err != nil {
		return nil, err
	}
	return readRedisValue(reader)
}

func readRedisValue(reader *bufio.Reader) (interface{}, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+':
		return line, nil
	case ':':
		return strconv.ParseInt(line, 10, 64)
	case '$':
		length, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if length < 0 {
			return "", nil
		}
		buf := make([]byte, length+2)
		if _, err := reader.Read(buf); err != nil {
			return nil, err
		}
		return string(buf[:length]), nil
	case '*':
		length, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if length < 0 {
			return []interface{}{}, nil
		}
		items := make([]interface{}, 0, length)
		for i := 0; i < length; i++ {
			item, err := readRedisValue(reader)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	case '-':
		return nil, errors.New(line)
	default:
		return nil, fmt.Errorf("unsupported redis response prefix %q", prefix)
	}
}

func redisInt(value interface{}) int64 {
	if number, ok := value.(int64); ok {
		return number
	}
	return 0
}

func redisStringList(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, fmt.Sprint(item))
	}
	return result
}

func redisValueString(value interface{}) string {
	if items, ok := value.([]interface{}); ok {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprint(value)
}

func parseRedisInfo(info string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if ok {
			result[key] = value
		}
	}
	return result
}
