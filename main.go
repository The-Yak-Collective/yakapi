package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rhettg/batteries/yakapi/internal/ci"
	mw "github.com/rhettg/batteries/yakapi/internal/mw"
	"tailscale.com/client/tailscale"
)

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "/v1")
	w.WriteHeader(http.StatusTemporaryRedirect)
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
		Name: "batteries_processed_ops_total",
		Help: "The total number of processed requests",
	})
)

func errorResponse(w http.ResponseWriter, respErr error, statusCode int) error {
	resp := struct {
		Error string `json:"error"`
	}{Error: respErr.Error()}

	return sendResponse(w, resp, statusCode)
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
	whois, err := tailscale.WhoIs(r.Context(), r.RemoteAddr)
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
	if r.Method != "POST" {
		errorResponse(w, errors.New("POST required"), http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("content-type") != "application/json" {
		errorResponse(w, errors.New("application/json required"), http.StatusUnsupportedMediaType)
		return
	}

	req := struct {
		Command string `json:"command"`
	}{}

	err := json.NewDecoder(r.Body).Decode(&req)
	defer r.Body.Close()

	if err != nil {
		log.Errorw("failed parsing body", "error", err)
		errorResponse(w, errors.New("failed parsing body"), http.StatusBadRequest)
		return
	}

	err = ci.Accept(r.Context(), req.Command)
	if err != nil {
		log.Errorw("failed accepting ci command", "error", err)
		errorResponse(w, err, http.StatusBadRequest)
		return
	}

	resp := struct {
		Result string `json:"result"`
	}{
		Result: "ok",
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

	content, err := ioutil.ReadFile(captureFile)
	if err != nil {
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func homev1(w http.ResponseWriter, r *http.Request) {
	opsProcessed.Inc()

	resp := struct {
		Name      string     `json:"name"`
		UpTime    int64      `json:"uptime"`
		Resources []resource `json:"resources"`
	}{
		Name:   "Batteries Not Included",
		UpTime: int64(time.Since(startTime).Seconds()),
		Resources: []resource{
			{Name: "operator", Ref: "https://t.me/rhettg"},
			{Name: "project", Ref: "https://github.com/rhettg/batteries"},
			{Name: "metrics", Ref: "/metrics"},
		},
	}

	err := sendResponse(w, resp, http.StatusOK)
	if err != nil {
		log.Errorw("error sending response", "error", err)
		return
	}
}

var log *zap.SugaredLogger

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync() // flushes buffer, if any
	log = logger.Sugar()
	log.Infow("starting", "version", "1.0.0")

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "batteries_uptime_seconds",
		Help: "The uptime of the batteries service",
	}, func() float64 {
		return float64(time.Since(startTime).Seconds())
	})

	logmw := mw.New(logger)

	http.Handle("/", logmw(http.HandlerFunc(home)))
	http.Handle("/v1", logmw(http.HandlerFunc(homev1)))
	http.Handle("/v1/me", logmw(http.HandlerFunc(me)))
	http.Handle("/v1/ci", logmw(http.HandlerFunc(handleCI)))
	http.Handle("/metrics", logmw(promhttp.Handler()))

	http.ListenAndServe(":8080", nil)
}
