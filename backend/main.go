package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type ServiceItem struct {
	ID          int       `json:"id"`
	ServiceName string    `json:"service_name"`
	OwnerTeam   string    `json:"owner_team"`
	StorageType string    `json:"storage_type"`
	CreatedAt   time.Time `json:"created_at"`
}

var (
	storageMode string
	studentNIM  string
	studentName string
	db          *sql.DB
	memMutex    sync.Mutex
	memStore    []ServiceItem
)

func main() {
	appPort := getEnv("APP_PORT", "8080")
	storageMode = getEnv("STORAGE_MODE", "memory")
	studentNIM = getEnv("STUDENT_NIM", "42430003")
	studentName = getEnv("STUDENT_NAME", "I Made Obi Pranata")

	memStore = []ServiceItem{
		{
			ID:          1,
			ServiceName: "Standalone-API-Demo",
			OwnerTeam:   fmt.Sprintf("%s (%s)", studentName, studentNIM),
			StorageType: "in-memory",
			CreatedAt:   time.Now(),
		},
	}

	if storageMode == "postgres" {
		initPostgres()
		defer db.Close()
	} else {
		log.Printf("[INFO] Mahasiswa %s (%s) menjalankan mode MANDIRI (STORAGE_MODE=memory) tanpa DB.", studentName, studentNIM)
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		hostname, _ := os.Hostname()

		if storageMode == "postgres" {
			if err := db.Ping(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]string{
					"status":               "unhealthy",
					"student_nim":          studentNIM,
					"student_name":         studentName,
					"mode":                 storageMode,
					"container_or_host_id": hostname,
					"error":                err.Error(),
				})
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":               "healthy",
			"student_nim":          studentNIM,
			"student_name":         studentName,
			"mode":                 storageMode,
			"container_or_host_id": hostname,
		})
	})

	http.HandleFunc("/api/services", handleServices)

	log.Printf("[INFO] Server Backend API aktif di port :%s (Mode: %s | NIM: %s)", appPort, storageMode, studentNIM)
	log.Fatal(http.ListenAndServe(":"+appPort, nil))
}

func initPostgres() {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "devops_user"),
		getEnv("DB_PASSWORD", "devops_secret"),
		getEnv("DB_NAME", "devops_inventory"),
	)
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("[FATAL] Gagal membuka driver PostgreSQL: %v", err)
	}
	log.Println("[INFO] Terhubung ke mode penyimpanan PostgreSQL (STORAGE_MODE=postgres).")
}

func handleServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		if storageMode == "postgres" {
			rows, err := db.Query("SELECT id, service_name, owner_team, created_at FROM services ORDER BY id DESC")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()
			items := []ServiceItem{}
			for rows.Next() {
				var s ServiceItem
				if err := rows.Scan(&s.ID, &s.ServiceName, &s.OwnerTeam, &s.CreatedAt); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				s.StorageType = "postgresql-persistent"
				items = append(items, s)
			}
			json.NewEncoder(w).Encode(items)
			return
		}
		memMutex.Lock()
		defer memMutex.Unlock()
		json.NewEncoder(w).Encode(memStore)
		return
	}

	if r.Method == http.MethodPost {
		var input struct {
			ServiceName string `json:"service_name"`
			OwnerTeam   string `json:"owner_team"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.ServiceName == "" {
			http.Error(w, "Format JSON tidak valid", http.StatusBadRequest)
			return
		}

		if storageMode == "postgres" {
			var s ServiceItem
			err := db.QueryRow(
				"INSERT INTO services (service_name, owner_team) VALUES ($1, $2) RETURNING id, service_name, owner_team, created_at",
				input.ServiceName, input.OwnerTeam,
			).Scan(&s.ID, &s.ServiceName, &s.OwnerTeam, &s.CreatedAt)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.StorageType = "postgresql-persistent"
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(s)
			return
		}

		memMutex.Lock()
		newItem := ServiceItem{
			ID:          len(memStore) + 1,
			ServiceName: input.ServiceName,
			OwnerTeam:   input.OwnerTeam,
			StorageType: "in-memory",
			CreatedAt:   time.Now(),
		}
		memStore = append(memStore, newItem)
		memMutex.Unlock()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(newItem)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
