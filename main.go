package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"URLshortner/Internal/storage"
	"URLshortner/assets"
	"URLshortner/ui/pages"

	"github.com/a-h/templ"
	"github.com/joho/godotenv"
)

func main() {
	InitDotEnv()

	// Initialize database
	db, err := storage.InitDB("urlshortener.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create store
	store := storage.New(db)

	mux := http.NewServeMux()
	SetupAssetsRoutes(mux)
	mux.Handle("GET /", templ.Handler(pages.Landing()))
	mux.HandleFunc("POST /shorten", MakeEncodeURL(store))
	mux.HandleFunc("GET /{encodedIndex}", MakeRedirectURL(store))
	fmt.Println("Server is running on http://localhost:8090")
	log.Fatal(http.ListenAndServe(":8090", mux))
}

func InitDotEnv() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}
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
