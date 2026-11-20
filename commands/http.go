// DESC: Local HTTP webhook inspector, mock/chaos server, static file server & request replayer
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"
)

type Style struct {
	Bold    string
	Dim     string
	Cyan    string
	Green   string
	Yellow  string
	Red     string
	Blue    string
	Magenta string
	Reset   string
}

func newStyle(noColor bool) Style {
	if noColor || os.Getenv("NO_COLOR") != "" || !isStdoutTTY() {
		return Style{}
	}
	return Style{
		Bold:    "\033[1m",
		Dim:     "\033[2m",
		Cyan:    "\033[36m",
		Green:   "\033[32m",
		Yellow:  "\033[33m",
		Red:     "\033[31m",
		Blue:    "\033[34m",
		Magenta: "\033[35m",
		Reset:   "\033[0m",
	}
}

func isStdoutTTY() bool {
	info, err := os.Stdout.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

type Config struct {
	Host        string
	Port        int
	StrictPort  bool
	ServeDir    string
	SPA         bool
	CORS        bool
	NoCache     bool
	BasicAuth   string
	Status      int
	Body        string
	BodyFile    string
	Headers     []string
	Delay       time.Duration
	Jitter      time.Duration
	FailRate    float64
	FailStatus  int
	SaveDir     string
	ShowHeaders bool
	headersSet  bool
	Compact     bool
	MaxBody     int
	OpenBrowser bool
	NoColor     bool
}

type CapturedRequest struct {
	ID         uint64              `json:"id"`
	Timestamp  string              `json:"timestamp"`
	Method     string              `json:"method"`
	Path       string              `json:"path"`
	Query      map[string][]string `json:"query,omitempty"`
	Headers    map[string][]string `json:"headers"`
	RemoteAddr string              `json:"remote_addr"`
	Body       string              `json:"body,omitempty"`
	BodyBase64 string              `json:"body_base64,omitempty"`
	JSON       any                 `json:"json,omitempty"`
}

var (
	st         Style
	reqCounter atomic.Uint64
	logMu      sync.Mutex
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "replay" {
		st = newStyle(false)
		runReplay(args[1:])
		return
	}

	cfg := parseArgs(args)
	st = newStyle(cfg.NoColor)

	if cfg.ServeDir != "" {
		info, err := os.Stat(cfg.ServeDir)
		if err != nil || !info.IsDir() {
			fatal(fmt.Sprintf("serve path '%s' is not a valid directory", cfg.ServeDir))
		}
	}

	if cfg.BodyFile != "" {
		if _, err := os.Stat(cfg.BodyFile); err != nil {
			fatal(fmt.Sprintf("body file '%s' cannot be read: %v", cfg.BodyFile, err))
		}
	}

	if cfg.SaveDir != "" {
		if err := os.MkdirAll(cfg.SaveDir, 0o755); err != nil {
			fatal(fmt.Sprintf("failed to create save directory '%s': %v", cfg.SaveDir, err))
		}
	}

	listener, actualPort, err := bindListener(cfg.Host, cfg.Port, cfg.StrictPort)
	if err != nil {
		fatal(err.Error())
	}
	defer func() { _ = listener.Close() }()

	handler := buildHandler(cfg)
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
	}

	printBanner(cfg, actualPort)

	localURL := fmt.Sprintf("http://localhost:%d", actualPort)
	if cfg.OpenBrowser {
		openURL(localURL)
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		if serveErr := srv.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	select {
	case sig := <-stopCh:
		fmt.Printf("\n%sReceived %s — shutting down (handled %d request(s)).%s\n",
			st.Dim, sig, reqCounter.Load(), st.Reset)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	case serveErr := <-errCh:
		fatal(serveErr.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "%sError:%s %s\n", st.Red, st.Reset, msg)
	os.Exit(1)
}

func parseArgs(args []string) Config {
	cfg := Config{
		Host:       "0.0.0.0",
		Port:       8080,
		CORS:       true,
		Status:     http.StatusOK,
		FailStatus: http.StatusServiceUnavailable,
		MaxBody:    65536,
	}

	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		nextVal := func(flagName string) string {
			if i+1 >= len(args) {
				fatal(fmt.Sprintf("flag %s requires a value", flagName))
			}
			i++
			return args[i]
		}

		switch {
		case arg == "-h" || arg == "--help" || arg == "help":
			st = newStyle(false)
			printHelp()
			os.Exit(0)
		case arg == "-p" || arg == "--port":
			cfg.Port = parsePort(nextVal(arg))
		case strings.HasPrefix(arg, "--port="):
			cfg.Port = parsePort(strings.TrimPrefix(arg, "--port="))
		case arg == "--host":
			cfg.Host = nextVal(arg)
		case arg == "--local":
			cfg.Host = "127.0.0.1"
		case arg == "--strict-port":
			cfg.StrictPort = true
		case arg == "-s" || arg == "--serve":
			cfg.ServeDir = nextVal(arg)
		case strings.HasPrefix(arg, "--serve="):
			cfg.ServeDir = strings.TrimPrefix(arg, "--serve=")
		case arg == "--spa":
			cfg.SPA = true
		case arg == "--cors":
			cfg.CORS = true
		case arg == "--no-cors":
			cfg.CORS = false
		case arg == "--no-cache":
			cfg.NoCache = true
		case arg == "--auth":
			cfg.BasicAuth = nextVal(arg)
		case arg == "--status":
			cfg.Status = parseIntFlag(arg, nextVal(arg), 100, 599)
		case strings.HasPrefix(arg, "--status="):
			cfg.Status = parseIntFlag("--status", strings.TrimPrefix(arg, "--status="), 100, 599)
		case arg == "--body":
			cfg.Body = nextVal(arg)
		case strings.HasPrefix(arg, "--body="):
			cfg.Body = strings.TrimPrefix(arg, "--body=")
		case arg == "--body-file":
			cfg.BodyFile = nextVal(arg)
		case arg == "-H" || arg == "--header":
			cfg.Headers = append(cfg.Headers, nextVal(arg))
		case arg == "--delay":
			cfg.Delay = parseDurationFlag(arg, nextVal(arg))
		case strings.HasPrefix(arg, "--delay="):
			cfg.Delay = parseDurationFlag("--delay", strings.TrimPrefix(arg, "--delay="))
		case arg == "--jitter":
			cfg.Jitter = parseDurationFlag(arg, nextVal(arg))
		case arg == "--fail-rate":
			cfg.FailRate = parseFloatFlag(arg, nextVal(arg), 0.0, 1.0)
		case arg == "--fail-status":
			cfg.FailStatus = parseIntFlag(arg, nextVal(arg), 100, 599)
		case arg == "--save":
			cfg.SaveDir = nextVal(arg)
		case arg == "--headers":
			cfg.ShowHeaders = true
			cfg.headersSet = true
		case arg == "--no-headers":
			cfg.ShowHeaders = false
			cfg.headersSet = true
		case arg == "--compact":
			cfg.Compact = true
		case arg == "--max-body":
			cfg.MaxBody = parseIntFlag(arg, nextVal(arg), 0, 100_000_000)
		case arg == "--open":
			cfg.OpenBrowser = true
		case arg == "--no-color":
			cfg.NoColor = true
		case strings.HasPrefix(arg, "-"):
			fatal(fmt.Sprintf("unknown option '%s' (run 'dev http --help')", arg))
		default:
			positionals = append(positionals, arg)
		}
	}

	for _, pos := range positionals {
		if p, err := strconv.Atoi(pos); err == nil {
			if p < 1 || p > 65535 {
				fatal("port must be between 1 and 65535")
			}
			cfg.Port = p
		} else if info, statErr := os.Stat(pos); statErr == nil && info.IsDir() {
			cfg.ServeDir = pos
		} else {
			fatal(fmt.Sprintf("unexpected argument '%s' (expected port number or existing directory)", pos))
		}
	}

	if !cfg.headersSet {
		cfg.ShowHeaders = cfg.ServeDir == ""
	}

	return cfg
}

func parsePort(raw string) int {
	return parseIntFlag("--port", raw, 1, 65535)
}

func parseIntFlag(name, raw string, minVal, maxVal int) int {
	v, err := strconv.Atoi(raw)
	if err != nil || v < minVal || v > maxVal {
		fatal(fmt.Sprintf("%s must be an integer between %d and %d", name, minVal, maxVal))
	}
	return v
}

func parseFloatFlag(name, raw string, minVal, maxVal float64) float64 {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < minVal || v > maxVal {
		fatal(fmt.Sprintf("%s must be a number between %.2f and %.2f", name, minVal, maxVal))
	}
	return v
}

func parseDurationFlag(name, raw string) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		fatal(fmt.Sprintf("%s must be a valid non-negative duration (e.g., 250ms, 2s)", name))
	}
	return d
}

func printHelp() {
	fmt.Printf("%sdev http%s — Local HTTP webhook inspector, mock server, static server & replayer\n\n", st.Bold, st.Reset)
	fmt.Printf("%sUsage:%s\n", st.Bold, st.Reset)
	fmt.Println("  dev http [port | dir] [options]")
	fmt.Println("  dev http replay <captured.json> <target-url>")
	fmt.Println()

	fmt.Printf("%sServer & Network Options:%s\n", st.Bold, st.Reset)
	fmt.Printf("  %s%-22s%s Port to listen on (default: 8080; auto-increments if busy)\n", st.Cyan, "-p, --port <port>", st.Reset)
	fmt.Printf("  %s%-22s%s Bind address (default: 0.0.0.0; use --local for 127.0.0.1)\n", st.Cyan, "--host <addr>", st.Reset)
	fmt.Printf("  %s%-22s%s Fail immediately if target port is already in use\n", st.Cyan, "--strict-port", st.Reset)
	fmt.Printf("  %s%-22s%s Open server URL in default browser on startup\n", st.Cyan, "--open", st.Reset)
	fmt.Println()

	fmt.Printf("%sStatic File & SPA Server Options:%s\n", st.Bold, st.Reset)
	fmt.Printf("  %s%-22s%s Serve files from <dir> (or pass directory path directly)\n", st.Cyan, "-s, --serve <dir>", st.Reset)
	fmt.Printf("  %s%-22s%s Fallback unknown routes to index.html for Single Page Apps\n", st.Cyan, "--spa", st.Reset)
	fmt.Printf("  %s%-22s%s Enable/disable CORS headers (default: enabled)\n", st.Cyan, "--cors | --no-cors", st.Reset)
	fmt.Printf("  %s%-22s%s Send Cache-Control: no-store headers on all responses\n", st.Cyan, "--no-cache", st.Reset)
	fmt.Printf("  %s%-22s%s Protect endpoints with HTTP Basic Auth\n", st.Cyan, "--auth <user:pass>", st.Reset)
	fmt.Println()

	fmt.Printf("%sMocking, Latency & Chaos Options:%s\n", st.Bold, st.Reset)
	fmt.Printf("  %s%-22s%s HTTP status code returned in echo mode (default: 200)\n", st.Cyan, "--status <code>", st.Reset)
	fmt.Printf("  %s%-22s%s Custom response body string\n", st.Cyan, "--body <text>", st.Reset)
	fmt.Printf("  %s%-22s%s Serve response body from file (re-read per request)\n", st.Cyan, "--body-file <path>", st.Reset)
	fmt.Printf("  %s%-22s%s Custom response header (repeatable)\n", st.Cyan, "-H, --header 'K: V'", st.Reset)
	fmt.Printf("  %s%-22s%s Artificial fixed latency before responding (e.g., 500ms, 2s)\n", st.Cyan, "--delay <duration>", st.Reset)
	fmt.Printf("  %s%-22s%s Random additional latency between 0 and <duration>\n", st.Cyan, "--jitter <duration>", st.Reset)
	fmt.Printf("  %s%-22s%s Randomly fail fraction (0.0-1.0) of requests with --fail-status\n", st.Cyan, "--fail-rate <float>", st.Reset)
	fmt.Printf("  %s%-22s%s Status code for simulated chaos failures (default: 503)\n", st.Cyan, "--fail-status <code>", st.Reset)
	fmt.Println()

	fmt.Printf("%sInspection & Capture Options:%s\n", st.Bold, st.Reset)
	fmt.Printf("  %s%-22s%s Save each incoming request as a JSON file in <dir>\n", st.Cyan, "--save <dir>", st.Reset)
	fmt.Printf("  %s%-22s%s Show or hide request headers in terminal output\n", st.Cyan, "--headers|--no-headers", st.Reset)
	fmt.Printf("  %s%-22s%s Single-line access log format (hide headers & body)\n", st.Cyan, "--compact", st.Reset)
	fmt.Printf("  %s%-22s%s Max body bytes printed to terminal (default: 65536; 0=unlimited)\n", st.Cyan, "--max-body <bytes>", st.Reset)
}

// ============================================================================
// Network & Startup Helpers
// ============================================================================

func bindListener(host string, startPort int, strict bool) (net.Listener, int, error) {
	maxTries := 20
	if strict {
		maxTries = 1
	}

	var lastErr error
	for offset := 0; offset < maxTries; offset++ {
		port := startPort + offset
		if port > 65535 {
			break
		}
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, port, nil
		}
		lastErr = err
	}
	return nil, 0, fmt.Errorf("failed to bind starting at %s:%d: %w", host, startPort, lastErr)
}

func detectLANIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var ips []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				ips = append(ips, v4.String())
			}
		}
	}
	return ips
}

func printBanner(cfg Config, port int) {
	modeLabel := "Webhook & Echo Inspector"
	if cfg.ServeDir != "" {
		absDir, _ := filepath.Abs(cfg.ServeDir)
		modeLabel = fmt.Sprintf("Static File Server (%s)", absDir)
	}

	fmt.Printf("%sdev http%s — %s%s%s\n", st.Bold, st.Reset, st.Cyan, modeLabel, st.Reset)
	if port != cfg.Port {
		fmt.Printf("  %sNote:%s Port %d was busy; bound to %d instead.\n", st.Yellow, st.Reset, cfg.Port, port)
	}
	fmt.Printf("  Local   : %shttp://localhost:%d%s\n", st.Green, port, st.Reset)
	if cfg.Host != "127.0.0.1" && cfg.Host != "localhost" {
		for _, ip := range detectLANIPs() {
			fmt.Printf("  Network : %shttp://%s:%d%s\n", st.Green, ip, port, st.Reset)
		}
	}
	if cfg.SaveDir != "" {
		fmt.Printf("  Capture : saving requests to %s%s%s\n", st.Cyan, cfg.SaveDir, st.Reset)
	}
	if cfg.Delay > 0 || cfg.Jitter > 0 {
		fmt.Printf("  Latency : delay=%s jitter=%s\n", cfg.Delay, cfg.Jitter)
	}
	if cfg.FailRate > 0 {
		fmt.Printf("  Chaos   : %.0f%% failure rate -> HTTP %d\n", cfg.FailRate*100, cfg.FailStatus)
	}
	fmt.Printf("%sPress Ctrl+C to stop.%s\n\n", st.Dim, st.Reset)
}

func openURL(target string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	_ = cmd.Start()
}

// ============================================================================
// HTTP Handler Pipeline
// ============================================================================

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(p []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	n, err := sr.ResponseWriter.Write(p)
	sr.bytes += n
	return n, err
}

func buildHandler(cfg Config) http.Handler {
	var fileHandler http.Handler
	if cfg.ServeDir != "" {
		fileHandler = buildStaticHandler(cfg)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := reqCounter.Add(1)

		// Apply global headers (CORS, Cache-Control, custom headers)
		if cfg.CORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
			reqHeaders := r.Header.Get("Access-Control-Request-Headers")
			if reqHeaders == "" {
				reqHeaders = "Content-Type, Authorization, X-Requested-With"
			}
			w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
		}
		if cfg.NoCache {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		}
		applyCustomHeaders(w.Header(), cfg.Headers)

		// Handle CORS preflight automatically
		if cfg.CORS && r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			logCompactAccess(id, r, http.StatusNoContent, 0, time.Since(start))
			return
		}

		// Optional Basic Auth enforcement
		if cfg.BasicAuth != "" && !checkBasicAuth(r, cfg.BasicAuth) {
			w.Header().Set("WWW-Authenticate", `Basic realm="dev-http"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			logCompactAccess(id, r, http.StatusUnauthorized, 13, time.Since(start))
			return
		}

		// Artificial latency + jitter
		applyLatency(cfg.Delay, cfg.Jitter)

		// Chaos fault injection
		if cfg.FailRate > 0 && shouldFail(cfg.FailRate) {
			msg := fmt.Sprintf(`{"error":"simulated_failure","status":%d}`+"\n", cfg.FailStatus)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(cfg.FailStatus)
			_, _ = w.Write([]byte(msg))
			logCompactAccess(id, r, cfg.FailStatus, len(msg), time.Since(start))
			return
		}

		// Static file mode
		if fileHandler != nil {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			fileHandler.ServeHTTP(rec, r)
			logCompactAccess(id, r, rec.status, rec.bytes, time.Since(start))
			return
		}

		// Echo / Webhook inspector mode
		handleEchoRequest(cfg, id, w, r, start)
	})
}

func buildStaticHandler(cfg Config) http.Handler {
	root := cfg.ServeDir
	fs := http.FileServer(http.Dir(root))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.SPA {
			cleanPath := filepath.Clean("/" + r.URL.Path)
			fullPath := filepath.Join(root, filepath.FromSlash(cleanPath))
			if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
				indexFile := filepath.Join(root, "index.html")
				if _, idxErr := os.Stat(indexFile); idxErr == nil {
					http.ServeFile(w, r, indexFile)
					return
				}
			}
		}
		fs.ServeHTTP(w, r)
	})
}

func applyCustomHeaders(h http.Header, headers []string) {
	for _, raw := range headers {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) == 2 {
			h.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func checkBasicAuth(r *http.Request, expected string) bool {
	parts := strings.SplitN(expected, ":", 2)
	if len(parts) != 2 {
		return false
	}
	user, pass, ok := r.BasicAuth()
	if !ok {
		return false
	}
	userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(parts[0])) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(parts[1])) == 1
	return userMatch && passMatch
}

func applyLatency(base, jitter time.Duration) {
	total := base
	if jitter > 0 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(jitter)))
		if err == nil {
			total += time.Duration(n.Int64())
		}
	}
	if total > 0 {
		time.Sleep(total)
	}
}

func shouldFail(rate float64) bool {
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	const scale = 1_000_000
	n, err := rand.Int(rand.Reader, big.NewInt(scale))
	if err != nil {
		return false
	}
	return float64(n.Int64())/float64(scale) < rate
}

func handleEchoRequest(cfg Config, id uint64, w http.ResponseWriter, r *http.Request, start time.Time) {
	bodyBytes, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()

	now := time.Now().UTC()
	captured := CapturedRequest{
		ID:         id,
		Timestamp:  now.Format(time.RFC3339Nano),
		Method:     r.Method,
		Path:       r.URL.Path,
		Query:      r.URL.Query(),
		Headers:    r.Header,
		RemoteAddr: r.RemoteAddr,
	}

	if len(bodyBytes) > 0 {
		if utf8.Valid(bodyBytes) {
			captured.Body = string(bodyBytes)
			var parsedJSON any
			if json.Unmarshal(bodyBytes, &parsedJSON) == nil {
				captured.JSON = parsedJSON
			}
		} else {
			captured.BodyBase64 = base64.StdEncoding.EncodeToString(bodyBytes)
		}
	}

	if cfg.SaveDir != "" {
		saveCapturedRequest(cfg.SaveDir, captured)
	}

	// Built-in lightweight health endpoint when no custom body is configured
	if r.URL.Path == "/__health" && cfg.Body == "" && cfg.BodyFile == "" {
		resp := `{"status":"ok"}` + "\n"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
		logEchoRequest(cfg, captured, http.StatusOK, bodyBytes, time.Since(start))
		return
	}

	respBytes, contentType := buildEchoResponse(cfg, captured)
	if w.Header().Get("Content-Type") == "" && contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(cfg.Status)
	_, _ = w.Write(respBytes)

	logEchoRequest(cfg, captured, cfg.Status, bodyBytes, time.Since(start))
}

func buildEchoResponse(cfg Config, captured CapturedRequest) ([]byte, string) {
	if cfg.BodyFile != "" {
		data, err := os.ReadFile(cfg.BodyFile)
		if err != nil {
			return []byte(fmt.Sprintf(`{"error":"failed to read body file: %s"}`+"\n", err)), "application/json"
		}
		return data, detectContentType(data)
	}

	if cfg.Body != "" {
		data := []byte(cfg.Body)
		if !strings.HasSuffix(cfg.Body, "\n") {
			data = append(data, '\n')
		}
		return data, detectContentType(data)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(captured)
	return buf.Bytes(), "application/json; charset=utf-8"
}

func detectContentType(data []byte) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid(trimmed) {
		return "application/json; charset=utf-8"
	}
	return http.DetectContentType(data)
}

func saveCapturedRequest(dir string, captured CapturedRequest) {
	ts := time.Now().UTC().Format("20060102T150405Z")
	filename := fmt.Sprintf("req-%04d-%s-%s.json", captured.ID, ts, captured.Method)
	fullPath := filepath.Join(dir, filename)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(captured); err != nil {
		return
	}
	_ = os.WriteFile(fullPath, buf.Bytes(), 0o644)
}

// ============================================================================
// Terminal Logging & JSON Syntax Highlighting
// ============================================================================

func methodColor(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return st.Green
	case http.MethodPost:
		return st.Cyan
	case http.MethodPut, http.MethodPatch:
		return st.Yellow
	case http.MethodDelete:
		return st.Red
	default:
		return st.Magenta
	}
}

func statusColor(code int) string {
	switch {
	case code >= 500:
		return st.Red
	case code >= 400:
		return st.Yellow
	case code >= 300:
		return st.Cyan
	default:
		return st.Green
	}
}

func logCompactAccess(id uint64, r *http.Request, status, size int, dur time.Duration) {
	logMu.Lock()
	defer logMu.Unlock()

	ts := time.Now().Format("15:04:05")
	mCol := methodColor(r.Method)
	sCol := statusColor(status)
	fmt.Printf("%s[%s]%s %s#%d%s %s%-7s%s %s -> %s%d%s (%dB, %s)\n",
		st.Dim, ts, st.Reset,
		st.Dim, id, st.Reset,
		mCol, r.Method, st.Reset,
		r.URL.RequestURI(),
		sCol, status, st.Reset,
		size, dur.Round(time.Microsecond),
	)
}

func logEchoRequest(cfg Config, captured CapturedRequest, status int, rawBody []byte, dur time.Duration) {
	if cfg.Compact {
		logMu.Lock()
		defer logMu.Unlock()
		ts := time.Now().Format("15:04:05")
		fmt.Printf("%s[%s]%s %s#%d%s %s%-7s%s %s -> %s%d%s (%dB, %s)\n",
			st.Dim, ts, st.Reset,
			st.Dim, captured.ID, st.Reset,
			methodColor(captured.Method), captured.Method, st.Reset,
			captured.Path,
			statusColor(status), status, st.Reset,
			len(rawBody), dur.Round(time.Microsecond),
		)
		return
	}

	logMu.Lock()
	defer logMu.Unlock()

	ts := time.Now().Format("15:04:05")
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", st.Dim, st.Reset)
	fmt.Printf("%s[%s]%s %s#%d%s %s%s%s %s%s%s  ->  %s%d%s %s(%s, %s)%s\n",
		st.Dim, ts, st.Reset,
		st.Bold, captured.ID, st.Reset,
		methodColor(captured.Method)+st.Bold, captured.Method, st.Reset,
		st.Bold, captured.Path, st.Reset,
		statusColor(status), status, st.Reset,
		st.Dim, captured.RemoteAddr, dur.Round(time.Microsecond), st.Reset,
	)

	if len(captured.Query) > 0 {
		fmt.Printf("%sQuery:%s\n", st.Bold, st.Reset)
		keys := sortedMapKeys(captured.Query)
		for _, k := range keys {
			fmt.Printf("  %s%s%s = %s\n", st.Cyan, k, st.Reset, strings.Join(captured.Query[k], ", "))
		}
	}

	if cfg.ShowHeaders && len(captured.Headers) > 0 {
		fmt.Printf("%sHeaders:%s\n", st.Bold, st.Reset)
		keys := sortedMapKeys(captured.Headers)
		for _, k := range keys {
			fmt.Printf("  %s%-22s%s : %s\n", st.Dim, k, st.Reset, strings.Join(captured.Headers[k], ", "))
		}
	}

	if len(rawBody) > 0 {
		fmt.Printf("%sBody (%d bytes):%s\n", st.Bold, len(rawBody), st.Reset)
		displayBody := rawBody
		truncated := false
		if cfg.MaxBody > 0 && len(displayBody) > cfg.MaxBody {
			displayBody = displayBody[:cfg.MaxBody]
			truncated = true
		}

		contentType := ""
		if vals, ok := captured.Headers["Content-Type"]; ok && len(vals) > 0 {
			contentType = strings.ToLower(vals[0])
		}

		switch {
		case json.Valid(displayBody):
			fmt.Println(formatColorJSON(displayBody))
		case strings.Contains(contentType, "application/x-www-form-urlencoded"):
			if parsedForm, err := url.ParseQuery(string(displayBody)); err == nil && len(parsedForm) > 0 {
				for _, k := range sortedMapKeys(parsedForm) {
					fmt.Printf("  %s%s%s = %s\n", st.Cyan, k, st.Reset, strings.Join(parsedForm[k], ", "))
				}
			} else {
				fmt.Println(string(displayBody))
			}
		case utf8.Valid(displayBody):
			fmt.Println(string(displayBody))
		default:
			fmt.Printf("  %s<binary payload: %d bytes>%s\n", st.Dim, len(rawBody), st.Reset)
		}

		if truncated {
			fmt.Printf("  %s... (truncated at %d bytes; pass --max-body 0 to show all)%s\n",
				st.Yellow, cfg.MaxBody, st.Reset)
		}
	}
	fmt.Println()
}

func sortedMapKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatColorJSON(raw []byte) string {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return string(raw)
	}
	if st.Reset == "" {
		return pretty.String()
	}

	dec := json.NewDecoder(bytes.NewReader(pretty.Bytes()))
	dec.UseNumber()

	var out bytes.Buffer
	type frame struct {
		isObject  bool
		expectKey bool
		count     int
	}
	var stack []frame

	writeIndent := func(depth int) {
		out.WriteByte('\n')
		for i := 0; i < depth; i++ {
			out.WriteString("  ")
		}
	}

	beforeValue := func() {
		if len(stack) == 0 {
			return
		}
		top := &stack[len(stack)-1]
		if top.isObject {
			if top.expectKey {
				if top.count > 0 {
					out.WriteByte(',')
				}
				writeIndent(len(stack))
			}
		} else {
			if top.count > 0 {
				out.WriteByte(',')
			}
			writeIndent(len(stack))
		}
	}

	afterValue := func() {
		if len(stack) == 0 {
			return
		}
		top := &stack[len(stack)-1]
		if top.isObject {
			if top.expectKey {
				out.WriteString(": ")
				top.expectKey = false
			} else {
				top.expectKey = true
				top.count++
			}
		} else {
			top.count++
		}
	}

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return pretty.String()
		}

		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				beforeValue()
				out.WriteByte('{')
				stack = append(stack, frame{isObject: true, expectKey: true})
			case '}':
				hadItems := len(stack) > 0 && stack[len(stack)-1].count > 0
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				if hadItems {
					writeIndent(len(stack))
				}
				out.WriteByte('}')
				afterValue()
			case '[':
				beforeValue()
				out.WriteByte('[')
				stack = append(stack, frame{isObject: false})
			case ']':
				hadItems := len(stack) > 0 && stack[len(stack)-1].count > 0
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				if hadItems {
					writeIndent(len(stack))
				}
				out.WriteByte(']')
				afterValue()
			}
		case string:
			beforeValue()
			encoded, _ := json.Marshal(v)
			isKey := len(stack) > 0 && stack[len(stack)-1].isObject && stack[len(stack)-1].expectKey
			if isKey {
				_, _ = fmt.Fprintf(&out, "%s%s%s", st.Cyan, encoded, st.Reset)
			} else {
				_, _ = fmt.Fprintf(&out, "%s%s%s", st.Green, encoded, st.Reset)
			}
			afterValue()
		case json.Number:
			beforeValue()
			_, _ = fmt.Fprintf(&out, "%s%s%s", st.Yellow, v.String(), st.Reset)
			afterValue()
		case bool:
			beforeValue()
			_, _ = fmt.Fprintf(&out, "%s%t%s", st.Magenta, v, st.Reset)
			afterValue()
		case nil:
			beforeValue()
			_, _ = fmt.Fprintf(&out, "%snull%s", st.Dim, st.Reset)
			afterValue()
		}
	}

	return out.String()
}

// ============================================================================
// Captured Request Replay Subcommand
// ============================================================================

func runReplay(args []string) {
	if len(args) < 2 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: dev http replay <captured-request.json> <target-base-url>")
		fmt.Println("Example: dev http replay ./webhooks/req-0001.json http://localhost:3000")
		return
	}

	captureFile := args[0]
	targetBase := strings.TrimRight(args[1], "/")

	raw, err := os.ReadFile(captureFile)
	if err != nil {
		fatal(fmt.Sprintf("failed to read capture file: %v", err))
	}

	var captured CapturedRequest
	if err := json.Unmarshal(raw, &captured); err != nil {
		fatal(fmt.Sprintf("invalid capture JSON: %v", err))
	}

	targetURL := targetBase + captured.Path
	if len(captured.Query) > 0 {
		q := url.Values(captured.Query)
		targetURL += "?" + q.Encode()
	}

	var payload []byte
	if captured.BodyBase64 != "" {
		payload, err = base64.StdEncoding.DecodeString(captured.BodyBase64)
		if err != nil {
			fatal(fmt.Sprintf("failed to decode base64 body: %v", err))
		}
	} else {
		payload = []byte(captured.Body)
	}

	req, err := http.NewRequest(captured.Method, targetURL, bytes.NewReader(payload))
	if err != nil {
		fatal(fmt.Sprintf("failed to build request: %v", err))
	}

	for k, vals := range captured.Headers {
		if strings.EqualFold(k, "Host") || strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	fmt.Printf("%sReplaying%s %s%s%s %s -> %s%s%s\n",
		st.Bold, st.Reset,
		methodColor(captured.Method), captured.Method, st.Reset,
		captureFile,
		st.Cyan, targetURL, st.Reset,
	)

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fatal(fmt.Sprintf("replay request failed: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("%sResponse:%s %s%s%s (%s)\n",
		st.Bold, st.Reset,
		statusColor(resp.StatusCode), resp.Status, st.Reset,
		time.Since(start).Round(time.Millisecond),
	)
	if len(respBody) > 0 {
		if json.Valid(respBody) {
			fmt.Println(formatColorJSON(respBody))
		} else {
			fmt.Println(string(respBody))
		}
	}
}
