package health

import (
	"log"
	"net/http"
)

func PrintBanner(app, version, env string) {

	log.Printf("Application: %s | Version: %s | Namespace: %s", app, version, env)
}

func Start(bind string) {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("OK"))
	})
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("READY"))
	})
	log.Printf("server listening on %s", bind)
	if err := http.ListenAndServe(bind, nil); err != nil {
		log.Printf("health server error: %v", err)
	}
}
