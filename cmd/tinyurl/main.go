package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"syscall"
	"time"

	swgui "github.com/swaggest/swgui/v5cdn"
	"go.opentelemetry.io/contrib/bridges/otelslog"

	"lorde.tech/toys/commons"
	rl "lorde.tech/toys/rate_limiter"
	tiny "lorde.tech/toys/shortener"
)

const name = "lorde.tech/toys/tinyurl/main"
const SETUP_FAILURE = 1

//go:embed static/openapi.yaml
var openapi string
var logger slog.Logger

func main() {
	otelShutdown := setupOTelOrDie()
	defer otelShutdown(context.Background())

	shortener := tiny.NewShortener()
	shortener.LoadFromLog()

	apiLimiter, err := rl.NewRateLimiter(10)
	if err != nil {
		logger.Error("[FATAL] Failed to create general rate limiter", "error", err)
		syscall.Exit(SETUP_FAILURE)
	}

	translateLimiter, err := rl.NewRateLimiter(100)
	if err != nil {
		logger.Error("[FATAL] Failed to create url translation rate limiter", "error", err)
		syscall.Exit(SETUP_FAILURE)
	}

	// Compaction routine
	go func() {
		for range time.Tick(24 * time.Hour) {
			go shortener.CompactLog()
			go apiLimiter.Compact()
			go translateLimiter.Compact()
		}
	}()

	apiWrapper := func(f http.HandlerFunc) http.HandlerFunc {
		return apiLimiter.LimitByIP(f)
	}

	http.HandleFunc("GET /{id}", translateLimiter.LimitByIP(translateTinyUrl(shortener)))
	http.HandleFunc("GET /api/urls", apiWrapper(listTinyUrls(shortener)))
	http.HandleFunc("POST /api/urls", apiWrapper(createTinyUrl(shortener)))
	http.HandleFunc("GET /api/urls/{id}", apiWrapper(fetchTinyUrl(shortener)))
	http.HandleFunc("DELETE /api/urls/{id}", apiWrapper(deleteTinyUrl(shortener)))

	http.HandleFunc("/", redicrectToDocs)
	http.Handle("/api/docs/", swgui.New("Tiny URL", "/api/docs/openapi.yaml", "/api/docs/"))
	http.HandleFunc("/api/docs/openapi.yaml", serveOpenapi)

	logger.Info("Listening on port 1337")
	http.ListenAndServe("localhost:1337", nil)
}

func setupOTelOrDie() func(context.Context) error {
	log := log.New(
		os.Stdout,
		"[SERVER] ",
		log.LUTC|log.Ldate|log.Ltime|log.Lmsgprefix,
	)

	log.Println("Setting up OpenTelemetry...")
	otelShutdown, err := commons.SetupOTelSDK(context.Background())
	if err != nil {
		log.Fatal("[FATAL] OpenTelemetry setup failed", err)
	}
	logger = *otelslog.NewLogger(name)
	log.Println("OpenTelemetry OK!")
	return otelShutdown
}

func redicrectToDocs(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/api/docs/", http.StatusFound)
}

func serveOpenapi(w http.ResponseWriter, r *http.Request) {
	log := newLogger(r, "API/DOCS")
	log.Info("openapi.yaml")
	fmt.Fprint(w, openapi)
}

func translateTinyUrl(s *tiny.Shortener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := newLogger(r, "TRANSLATE")
		id := r.PathValue("id")
		url, err := s.Translate(id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			log.Error("Failed to process id \"%s\" -> %s\n", id, err.Error())
			url = "[NOT FOUND]"
		} else {
			w.Header().Set("Location", url)
			w.Header().Set("Cache-Control", "max-age=86400")
			w.WriteHeader(http.StatusFound)
		}
		log.Info("%s -> %s", id, url)
	}
}

func listTinyUrls(s *tiny.Shortener) http.HandlerFunc {
	const localhostIPV6 = "0000:0000:0000:0000:0000:0000:0000:0001"
	const localhostIPV4 = "0000:0000:0000:0000:0000:ffff:7f00:0001"
	return func(w http.ResponseWriter, r *http.Request) {
		log := newLogger(r, "API/LIST")
		if ip := commons.GetClientIp(r); ip != localhostIPV4 && ip != localhostIPV6 {
			log.Error("Forbidden when not from localhost")
			forbiddenRequest(w)
			return
		}
		log.Info("Starting...")
		res := []TinyUrlMapping{}
		for k, v := range s.ListAll() {
			res = append(res, TinyUrlMapping{From: k, To: v})
		}
		setCotentTypeToJson(w)
		json.NewEncoder(w).Encode(res)
		log.Info("Done!")
	}
}

func createTinyUrl(s *tiny.Shortener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := newLogger(r, "API/ADD")
		var body Request = Request{}
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&body)
		if err != nil {
			log.DefaultError(err)
			badRequest(w, err)
			return
		}
		bodyString, _ := json.Marshal(body)
		log.Info("body -> '%s'\n", string(bodyString))
		url := body.Target
		id := body.Id
		if id == "" {
			id, err = s.Insert(url)
		} else {
			err = s.InsertCustom(id, url)
		}

		defer func(s *tiny.Shortener) {
			if r := recover(); r != nil {
				_, err := s.Translate(id)
				if err == nil {
					err = s.Remove(id)
					if err != nil {
						log.Error("Could not recover from failed insertion: id -> %s error -> %s\n", id, err.Error())
					}
				}
				w.WriteHeader(http.StatusInternalServerError)
			}
		}(s)

		if err != nil {
			badRequest(w, err)
			id = "[ERROR]"
		} else {
			setCotentTypeToJson(w)
			json.NewEncoder(w).Encode(NewTinyUrlResponse{TinyUrl: id})
		}
		log.Info("%s -> %s", id, url)
	}
}

func deleteTinyUrl(s *tiny.Shortener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := newLogger(r, "API/DELETE")
		id := r.PathValue("id")
		log.Info("ID -> '%s'", id)
		_, err := s.Translate(id)
		if err != nil {
			log.DefaultError(err)
			badRequest(w, err)
			return
		}
		err = s.Remove(id)
		if err != nil {
			log.DefaultError(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		log.Info("Deleted ID '%s'", id)
	}
}

func fetchTinyUrl(s *tiny.Shortener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := newLogger(r, "API/FETCH")
		id := r.PathValue("id")
		target, err := s.Translate(id)
		if err != nil {
			log.DefaultError(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		setCotentTypeToJson(w)
		json.NewEncoder(w).Encode(TinyUrlMapping{From: id, To: target})
		log.Info("Fetched ID '%s'", id)
	}
}

type Logger struct {
	l *log.Logger
}

func newLogger(r *http.Request, tag string) *Logger {
	return &Logger{
		l: log.New(
			os.Stdout,
			fmt.Sprintf("[%s|%s] ", commons.GetClientIp(r), tag),
			log.LUTC|log.Ldate|log.Ltime|log.Lmsgprefix,
		),
	}

}

func (l *Logger) Info(format string, s ...any) {
	l.l.Printf(format+"\n", s...)
}

func (l *Logger) Error(format string, s ...any) {
	l.l.Printf("[ERROR] "+format+"\n", s...)
}

func (l *Logger) DefaultError(err error) {
	l.Error(err.Error())
}

func setCotentTypeToJson(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func forbiddenRequest(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
}

func badRequest(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	setCotentTypeToJson(w)
	json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
}

type Request struct {
	Id     string `json:"from,omitempty"`
	Target string `json:"to"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type NewTinyUrlResponse struct {
	TinyUrl string `json:"tiny_url"`
}

type TinyUrlMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}
