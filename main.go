package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"claude-monitor/internal/api"
	"claude-monitor/internal/monitor"
)

//go:embed static
var staticFS embed.FS

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

	// Initialize monitors
	processMonitor := monitor.NewProcessMonitor()
	tempMonitor := monitor.NewTemperatureMonitor()
	historyBuffer := monitor.NewHistoryBuffer()

	// Initialize API handler
	handler := api.NewHandler(processMonitor, tempMonitor, historyBuffer)

	// Create router
	mux := http.NewServeMux()

	// Register API routes
	handler.RegisterRoutes(mux)

	// Serve static files with no-cache headers
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("Failed to create static file system: %v", err)
	}
	fileServer := http.FileServer(http.FS(subFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	})

	// Start background polling
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Initial sample
		handler.RecordHistory()

		for range ticker.C {
			handler.RecordHistory()

			// Check alerts (could be extended to log or send notifications)
			cpuAlert, tempAlert, process := handler.CheckAlerts()
			if cpuAlert {
				log.Printf("ALERT: High CPU usage on process %s", process)
			}
			if tempAlert {
				log.Printf("ALERT: High temperature detected")
			}
		}
	}()

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting Claude Monitor on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
