package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"koletackney.dev/urlshortener/Internal/storage"
	"koletackney.dev/urlshortener/assets"
	"koletackney.dev/urlshortener/ui/pages"

	"github.com/a-h/templ"
	"github.com/joho/godotenv"
)

func main() {
	InitDotEnv()

	dbPath := getEnv("DB_PATH", "urlshortener.db")
	port := getEnv("PORT", "8090")

	// Initialize database
	db, err := storage.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
		}
	}(db)

	// Create store
	store := storage.New(db)

	mux := http.NewServeMux()
	SetupAssetsRoutes(mux)
	mux.Handle("GET /", templ.Handler(pages.Landing()))
	mux.HandleFunc("POST /shorten", MakeEncodeURL(store))
	mux.HandleFunc("GET /{encodedIndex}", MakeRedirectURL(store))
	addr := ":" + port
	fmt.Printf("Server is running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func InitDotEnv() {
	_ = godotenv.Load()
}

func SetupAssetsRoutes(mux *http.ServeMux) {
	var isDevelopment = os.Getenv("GO_ENV") != "production"

	assetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDevelopment {
			w.Header().Set("Cache-Control", "no-store")
		}

		var fs http.Handler
		if isDevelopment {
			fs = http.FileServer(http.Dir("./assets"))
		} else {
			fs = http.FileServer(http.FS(assets.Assets))
		}

		fs.ServeHTTP(w, r)
	})

	mux.Handle("GET /assets/", http.StripPrefix("/assets/", assetHandler))
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
