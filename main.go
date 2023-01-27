package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"go.uber.org/zap"

	"github.com/The-Yak-Collective/yakapi/internal/ci"
	mw "github.com/The-Yak-Collective/yakapi/internal/mw"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"tailscale.com/client/tailscale"
)

var (
	//go:embed assets/*
	assets embed.FS
)

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "/v1")
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func eyes(w http.ResponseWriter, r *http.Request) {
	content, err := assets.ReadFile("assets/index.html")
	if err != nil {
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	_, err = w.Write(content)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to write response:", err)
	}
}

type resource struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

var startTime time.Time

func init() {
	startTime = time.Now()
}

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "yakapi_processed_ops_total",
		Help: "The total number of processed requests",
	})
)

func errorResponse(w http.ResponseWriter, respErr error, statusCode int) {
	resp := struct {
		Error string `json:"error"`
	}{Error: respErr.Error()}

	if err := sendResponse(w, resp, statusCode); err != nil {
		fmt.Fprintln(os.Stderr, "failed to send error response:", err)
	}
}

func sendResponse(w http.ResponseWriter, resp interface{}, statusCode int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		return err
	}

	return nil
}

func me(w http.ResponseWriter, r *http.Request) {
	lc := tailscale.LocalClient{}
	whois, err := lc.WhoIs(r.Context(), r.RemoteAddr)
	if err != nil {
		errorResponse(w, errors.New("unknown"), http.StatusInternalServerError)
		log.Errorw("whois failure", "error", err)
		return
	}

	resp := struct {
		Name   string `json:"name"`
		Login  string `json:"login"`
		Device string `json:"device"`
	}{
		Name:   whois.UserProfile.DisplayName,
		Login:  whois.UserProfile.LoginName,
		Device: whois.Node.Hostinfo.Hostname(),
	}

	err = sendResponse(w, &resp, http.StatusOK)
	if err != nil {
		log.Errorw("error sending response", "err", err)
		return
	}
}

func handleCI(w http.ResponseWriter, r *http.Request) {
	adapter := os.Getenv("YAKAPI_CI_ADAPTER")
	if adapter == "" {
		err := errors.New("YAKAPI_CI_ADAPTER not configured")
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	if r.Method != "POST" {
		errorResponse(w, errors.New("POST required"), http.StatusMethodNotAllowed)
		return
	}

	// TODO: Maybe expose things like content-type as env variables

	defer r.Body.Close()
	output, err := ci.Accept(r.Context(), log, adapter, r.Body)
	if err != nil {
		log.Errorw("failed accepting ci command", "error", err)
		errorResponse(w, err, http.StatusBadRequest)
		return
	}

	resp := struct {
		Result string `json:"result"`
		Output string `json:"output"`
	}{
		Result: "ok",
		Output: output,
	}

	err = sendResponse(w, resp, http.StatusAccepted)
	if err != nil {
		log.Errorw("error sending response", "error", err)
		return
	}
}

func handleCamCapture(w http.ResponseWriter, r *http.Request) {
	captureFile := os.Getenv("YAKAPI_CAM_CAPTURE_PATH")
	if captureFile == "" {
		err := errors.New("YAKAPI_CAM_CAPTURE_PATH not configured")
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	content, err := os.ReadFile(captureFile)
	if err != nil {
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(content)
	if err != nil {
		fmt.Printf("failed writing response: %v", err)
	}
}

func homev1(w http.ResponseWriter, r *http.Request) {
	opsProcessed.Inc()

	resp := struct {
		Name      string     `json:"name"`
		Build     string     `json:"build"`
		UpTime    int64      `json:"uptime"`
		Resources []resource `json:"resources"`
	}{
		Name:   "YakAPI Server",
		Build:  revision,
		UpTime: int64(time.Since(startTime).Seconds()),
		Resources: []resource{
			{Name: "metrics", Ref: "/metrics"},
			{Name: "ci", Ref: "/v1/ci"},
			{Name: "cam", Ref: "/v1/cam/capture"},
		},
	}

	name := os.Getenv("YAKAPI_NAME")
	if name != "" {
		resp.Name = name
	}

	project := os.Getenv("YAKAPI_PROJECT")
	if project != "" {
		resp.Resources = append(resp.Resources, resource{Name: "project", Ref: project})
	}

	operator := os.Getenv("YAKAPI_OPERATOR")
	if operator != "" {
		resp.Resources = append(resp.Resources, resource{Name: "operator", Ref: operator})
	}

	err := sendResponse(w, resp, http.StatusOK)
	if err != nil {
		log.Errorw("error sending response", "error", err)
		return
	}
}

var log *zap.SugaredLogger
var revision = "unknown"

func main() {
	logger, _ := zap.NewDevelopment()

	defer func() {
		if err := logger.Sync(); err != nil {
			fmt.Fprintln(os.Stderr, "failed to sync logger", err)
		}
	}()

	log = logger.Sugar()

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "yakapi_uptime_seconds",
		Help: "The uptime of the yakapi service",
	}, func() float64 {
		return float64(time.Since(startTime).Seconds())
	})

	logmw := mw.New(logger)

	http.Handle("/", logmw(http.HandlerFunc(home)))
	http.Handle("/v1", logmw(http.HandlerFunc(homev1)))
	http.Handle("/v1/me", logmw(http.HandlerFunc(me)))
	http.Handle("/v1/ci", logmw(http.HandlerFunc(handleCI)))
	http.Handle("/v1/cam/capture", logmw(http.HandlerFunc(handleCamCapture)))
	http.Handle("/metrics", logmw(promhttp.Handler()))
	http.Handle("/eyes", logmw(http.HandlerFunc(eyes)))

	port := os.Getenv("YAKAPI_PORT")
	if port == "" {
		port = "8080"
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		log.Errorw("failed loading build info")
	}

	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
			break
		}
	}

	log.Infow("starting", "version", "1.0.0", "port", port, "build", revision)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.Errorw("error from ListenAndServer", "error", err)
	}
}
