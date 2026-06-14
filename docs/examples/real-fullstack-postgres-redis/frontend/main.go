package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const page = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>PAAP Orders Console</title>
  <style>
    body { margin: 0; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f5f7fb; color: #15202b; }
    main { max-width: 880px; margin: 0 auto; padding: 44px 20px; }
    section { background: #fff; border: 1px solid #d9e1ec; border-radius: 8px; padding: 28px; }
    h1 { margin: 0 0 8px; font-size: 30px; }
    p { color: #5f6f83; line-height: 1.55; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 14px; margin-top: 22px; }
    .card { border: 1px solid #e1e7ef; border-radius: 6px; padding: 16px; background: #fbfdff; }
    .label { font-size: 12px; text-transform: uppercase; color: #6b7a90; letter-spacing: .04em; }
    .value { margin-top: 8px; font-size: 20px; font-weight: 700; }
    .ok { color: #087f5b; }
    .bad { color: #b42318; }
    pre { white-space: pre-wrap; overflow-wrap: anywhere; background: #101828; color: #e6edf3; border-radius: 6px; padding: 14px; margin-top: 20px; }
    button { margin-top: 18px; height: 36px; border: 1px solid #1d4ed8; background: #1d4ed8; color: white; border-radius: 6px; padding: 0 14px; cursor: pointer; }
  </style>
</head>
<body>
<main>
  <section>
    <h1>PAAP Orders Console</h1>
    <p>Frontend component calls the backend component through a same-origin API proxy. The backend writes to PostgreSQL and Redis inside the Kubernetes environment.</p>
    <div class="grid">
      <div class="card"><div class="label">Frontend</div><div class="value ok">Running</div></div>
      <div class="card"><div class="label">Backend</div><div id="backend" class="value">Checking</div></div>
      <div class="card"><div class="label">PostgreSQL</div><div id="postgresql" class="value">Checking</div></div>
      <div class="card"><div class="label">Redis</div><div id="redis" class="value">Checking</div></div>
    </div>
    <button type="button" onclick="loadStatus()">Refresh</button>
    <pre id="raw">Waiting for backend status...</pre>
  </section>
</main>
<script>
async function loadStatus() {
  const raw = document.getElementById('raw');
  try {
    const res = await fetch('/api/status', { cache: 'no-store' });
    const data = await res.json();
    raw.textContent = JSON.stringify(data, null, 2);
    const checks = data.checks || [];
    const pg = checks.find((item) => item.name === 'postgresql');
    const redis = checks.find((item) => item.name === 'redis');
    const backendOk = res.ok && checks.every((item) => item.ok !== false);
    document.getElementById('backend').textContent = backendOk ? 'OK' : 'FAIL';
    document.getElementById('backend').className = backendOk ? 'value ok' : 'value bad';
    document.getElementById('postgresql').textContent = pg?.ok ? 'Read/write OK' : 'Failed';
    document.getElementById('postgresql').className = pg?.ok ? 'value ok' : 'value bad';
    document.getElementById('redis').textContent = redis?.ok ? 'SET/GET OK' : 'Failed';
    document.getElementById('redis').className = redis?.ok ? 'value ok' : 'value bad';
  } catch (err) {
    raw.textContent = String(err);
    document.getElementById('backend').textContent = 'Failed';
    document.getElementById('backend').className = 'value bad';
  }
}
loadStatus();
</script>
</body>
</html>`

func backendURL() string {
	value := strings.TrimRight(strings.TrimSpace(os.Getenv("BACKEND_URL")), "/")
	if value == "" {
		return "http://backend-1"
	}
	return value
}

func proxyStatus(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, backendURL()+"/api/status", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})
	mux.HandleFunc("/api/status", proxyStatus)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	log.Println("orders frontend listening on :80")
	log.Fatal(http.ListenAndServe(":80", mux))
}
