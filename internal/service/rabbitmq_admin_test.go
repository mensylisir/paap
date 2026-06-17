package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"paap/internal/k8s"
)

func TestRabbitMQAdminCreatesExchangeBindingAndPublishes(t *testing.T) {
	var requests []struct {
		Method string
		Path   string
		Body   map[string]interface{}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "user" || pass != "secret" {
			t.Fatalf("missing basic auth: user=%q ok=%v", user, ok)
		}
		item := struct {
			Method string
			Path   string
			Body   map[string]interface{}
		}{Method: r.Method, Path: r.URL.EscapedPath()}
		if r.Body != nil && r.ContentLength != 0 {
			_ = json.NewDecoder(r.Body).Decode(&item.Body)
		}
		requests = append(requests, item)
		if r.URL.EscapedPath() == "/api/exchanges/%2F/orders/publish" {
			_, _ = w.Write([]byte(`{"routed":true}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	info := k8s.RabbitMQConnectionInfo{ManagementURL: server.URL, Username: "user", Password: "secret"}
	if err := CreateRabbitMQExchange(t.Context(), info, "/", "orders", "topic", true); err != nil {
		t.Fatalf("CreateRabbitMQExchange: %v", err)
	}
	if err := CreateRabbitMQBinding(t.Context(), info, "/", "orders", "queue", "orders.created", "created.#", map[string]interface{}{"x-match": "all"}); err != nil {
		t.Fatalf("CreateRabbitMQBinding: %v", err)
	}
	result, err := PublishRabbitMQMessage(t.Context(), info, "/", "orders", "created.1", `{"id":1}`, map[string]interface{}{"content_type": "application/json"})
	if err != nil {
		t.Fatalf("PublishRabbitMQMessage: %v", err)
	}
	if !result.Routed {
		t.Fatalf("expected routed publish result")
	}

	if len(requests) != 3 {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].Method != http.MethodPut || requests[0].Path != "/api/exchanges/%2F/orders" {
		t.Fatalf("exchange request = %#v", requests[0])
	}
	if requests[0].Body["type"] != "topic" || requests[0].Body["durable"] != true {
		t.Fatalf("exchange payload = %#v", requests[0].Body)
	}
	if requests[1].Method != http.MethodPost || requests[1].Path != "/api/bindings/%2F/e/orders/q/orders.created" {
		t.Fatalf("binding request = %#v", requests[1])
	}
	if requests[1].Body["routing_key"] != "created.#" {
		t.Fatalf("binding payload = %#v", requests[1].Body)
	}
	if requests[2].Method != http.MethodPost || requests[2].Path != "/api/exchanges/%2F/orders/publish" {
		t.Fatalf("publish request = %#v", requests[2])
	}
	if requests[2].Body["payload"] != `{"id":1}` || requests[2].Body["payload_encoding"] != "string" {
		t.Fatalf("publish payload = %#v", requests[2].Body)
	}
}

func TestRabbitMQAdminQueueOperationsUseManagementAPI(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.EscapedPath())
		switch r.URL.EscapedPath() {
		case "/api/queues/%2F/jobs/get":
			_, _ = w.Write([]byte(`[{"payload":"hello","routing_key":"jobs","message_count":0,"properties":[]}]`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	info := k8s.RabbitMQConnectionInfo{ManagementURL: server.URL, Username: "user", Password: "secret"}
	if err := PurgeRabbitMQQueue(t.Context(), info, "/", "jobs"); err != nil {
		t.Fatalf("PurgeRabbitMQQueue: %v", err)
	}
	messages, err := GetRabbitMQMessages(t.Context(), info, "/", "jobs", 5, true)
	if err != nil {
		t.Fatalf("GetRabbitMQMessages: %v", err)
	}
	if len(messages) != 1 || messages[0].Payload != "hello" {
		t.Fatalf("messages = %#v", messages)
	}

	want := []string{
		"DELETE /api/queues/%2F/jobs/contents",
		"POST /api/queues/%2F/jobs/get",
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("path[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}
