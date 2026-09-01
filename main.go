package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"jelly-an-li/internal/config"
	"jelly-an-li/internal/updater"
)

// Version приложения (проставляется автоматически при сборке через ldflags)
var Version = "v1.0.0"

//go:embed all:frontend/dist
var assets embed.FS

func printHelp() {
	fmt.Println("Jellyfin Anime Linker")
	fmt.Println("\nUsage:")
	fmt.Println("  jelly-an-li serve [--port N]   - Run HTTP server (default)")
	fmt.Println("  jelly-an-li sync [--dry-run]  - Run one-time sync")
	fmt.Println("\nFlags for 'serve':")
	fmt.Println("  --port int    port to listen on (default 37773, or PORT env)")
	fmt.Println("\nFlags for 'sync':")
	fmt.Println("  --dry-run     dry run mode (preview changes without creating links)")
}

func main() {
	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	defaultPort := 37773
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			defaultPort = p
		}
	}
	portOpt := serveCmd.Int("port", defaultPort, "port to listen on")

	syncCmd := flag.NewFlagSet("sync", flag.ExitOnError)
	dryRunOpt := syncCmd.Bool("dry-run", false, "dry run mode")

	cmd := "serve"
	if len(os.Args) > 1 {
		if os.Args[1] == "-h" || os.Args[1] == "--help" {
			printHelp()
			return
		}
		if !strings.HasPrefix(os.Args[1], "-") {
			cmd = os.Args[1]
			switch cmd {
			case "serve":
				serveCmd.Parse(os.Args[2:])
			case "sync":
				syncCmd.Parse(os.Args[2:])
			default:
				fmt.Printf("Unknown command: %s\n", cmd)
				printHelp()
				os.Exit(1)
			}
		} else {
			serveCmd.Parse(os.Args[1:])
		}
	} else {
		serveCmd.Parse([]string{})
	}

	myApp := NewApp()
	myApp.startup()

	switch cmd {
	case "sync":
		fmt.Printf("Running CLI Sync (dry-run: %v)...\n", *dryRunOpt)
		myApp.RunSync(*dryRunOpt)
		fmt.Println("CLI Sync completed.")
	case "serve":
		myApp.StartBackgroundSync()
		defer myApp.shutdown()

		mux := http.NewServeMux()

		// API endpoints
		mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				json.NewEncoder(w).Encode(myApp.GetConfig())
				return
			}
			if r.Method == http.MethodPost {
				var cfg config.Config
				if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if err := myApp.SaveConfig(&cfg); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		})

		mux.HandleFunc("/api/sync", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			dryRun := r.URL.Query().Get("dry_run") == "true"
			go myApp.RunSync(dryRun)
			json.NewEncoder(w).Encode(map[string]string{"status": "started"})
		})

		mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				json.NewEncoder(w).Encode(map[string][]string{"logs": myApp.GetLogs()})
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		})

		eventsHandler := func(w http.ResponseWriter, r *http.Request) {
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")

			sub, unsubscribe := myApp.SubscribeEvents()
			defer unsubscribe()

			// Сразу отправляем клиенту текущий статус синхронизации при подключении
			initialStatus := fmt.Sprintf(`{"syncing":%t}`, myApp.IsSyncing())
			fmt.Fprintf(w, "event: status\ndata: %s\n\n", initialStatus)
			flusher.Flush()

			notify := r.Context().Done()
			for {
				select {
				case <-notify:
					return
				case statusData, ok := <-sub.StatusCh:
					if !ok {
						return
					}
					fmt.Fprintf(w, "event: status\ndata: %s\n\n", statusData)
					flusher.Flush()
				case logData, ok := <-sub.LogCh:
					if !ok {
						return
					}
					fmt.Fprintf(w, "event: log\ndata: %s\n\n", strings.ReplaceAll(logData, "\n", " "))
					flusher.Flush()
				case _, ok := <-sub.ResetCh:
					if !ok {
						return
					}
					fmt.Fprintf(w, "event: reset\ndata: {}\n\n")
					flusher.Flush()
				}
			}
		}

		mux.HandleFunc("/api/events", eventsHandler)
		mux.HandleFunc("/api/logs/stream", eventsHandler)

		mux.HandleFunc("/api/logs/clear", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			myApp.ClearLogs()
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		})

		mux.HandleFunc("/api/cache/stats", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			json.NewEncoder(w).Encode(myApp.GetCacheInfo())
		})

		mux.HandleFunc("/api/cache/clear", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				ClearMetadata bool `json:"clear_metadata"`
				ClearState    bool `json:"clear_state"`
				Resync        bool `json:"resync"`
				DryRun        bool `json:"dry_run"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)

			// Если ничего явно не указано, очищаем и метаданные, и состояние
			if !req.ClearMetadata && !req.ClearState {
				req.ClearMetadata = true
				req.ClearState = true
			}

			myApp.ClearCache(req.ClearMetadata, req.ClearState)

			if req.Resync {
				go myApp.RunSync(req.DryRun)
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"resync":  req.Resync,
				"dry_run": req.DryRun,
			})
		})

		mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			json.NewEncoder(w).Encode(map[string]bool{"syncing": myApp.IsSyncing()})
		})

		mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			forceCheck := r.URL.Query().Get("force") == "true"
			info := updater.CheckUpdate(Version, forceCheck)
			json.NewEncoder(w).Encode(info)
		})

		mux.HandleFunc("/api/browse", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			pathQuery := r.URL.Query().Get("path")
			var targetPath string

			if pathQuery == "" {
				home, err := os.UserHomeDir()
				if err == nil {
					targetPath = home
				} else {
					targetPath = "/"
				}
			} else {
				targetPath = filepath.Clean(pathQuery)
			}

			entries, err := os.ReadDir(targetPath)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to read directory: %v", err), http.StatusInternalServerError)
				return
			}

			var directories []string
			for _, entry := range entries {
				if entry.IsDir() {
					if !strings.HasPrefix(entry.Name(), ".") {
						directories = append(directories, entry.Name())
					}
				}
			}

			sort.Strings(directories)

			parentPath := filepath.Dir(targetPath)
			if parentPath == targetPath {
				parentPath = ""
			}

			response := map[string]interface{}{
				"current_path": targetPath,
				"parent_path":  parentPath,
				"directories":  directories,
			}

			json.NewEncoder(w).Encode(response)
		})

		// Frontend static file serving
		distFS, err := fs.Sub(assets, "frontend/dist")
		if err != nil {
			log.Fatal(err)
		}
		fileServer := http.FileServer(http.FS(distFS))

		http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				mux.ServeHTTP(w, r)
				return
			}

			filePath := strings.TrimPrefix(r.URL.Path, "/")
			if filePath == "" {
				filePath = "index.html"
			}
			f, err := distFS.Open(filePath)
			if err != nil {
				r.URL.Path = "/"
			} else {
				f.Close()
			}

			fileServer.ServeHTTP(w, r)
		}))

		addr := fmt.Sprintf(":%d", *portOpt)
		fmt.Printf("Server is running on http://localhost%s\n", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatal("ListenAndServe error:", err)
		}
	}
}
