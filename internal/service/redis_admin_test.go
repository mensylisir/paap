package service

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadRedisValueSupportsArrays(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("*2\r\n$1\r\n0\r\n*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"))
	value, err := readRedisValue(reader)
	if err != nil {
		t.Fatalf("read redis array: %v", err)
	}
	array, ok := value.([]interface{})
	if !ok || len(array) != 2 {
		t.Fatalf("unexpected array %#v", value)
	}
	cursor, ok := array[0].(string)
	if !ok || cursor != "0" {
		t.Fatalf("unexpected cursor %#v", array[0])
	}
	keys, ok := array[1].([]interface{})
	if !ok || len(keys) != 2 || keys[0] != "foo" || keys[1] != "bar" {
		t.Fatalf("unexpected keys %#v", array[1])
	}
}

func TestRedisStringListConvertsRESPArray(t *testing.T) {
	got := redisStringList([]interface{}{"foo", "bar", int64(3)})
	if strings.Join(got, ",") != "foo,bar,3" {
		t.Fatalf("unexpected string list %#v", got)
	}
}
