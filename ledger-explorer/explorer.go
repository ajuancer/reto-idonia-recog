package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
)

const logDir = "/tmp/mylog"

type ExplorerData struct {
	Checkpoint string
	Logs       []string
}

func main() {
	_ = godotenv.Load(".env/.local")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1. Read the Checkpoint
		cpBytes, err := os.ReadFile(filepath.Join(logDir, "checkpoint"))
		cpText := string(cpBytes)
		if err != nil {
			cpText = fmt.Sprintf("Error reading checkpoint: %v", err)
		}

		logs := extractLogsFromTiles(filepath.Join(logDir, "tile"))

		data := ExplorerData{
			Checkpoint: cpText,
			Logs:       logs,
		}

		t, err := template.ParseFiles("index.html")
		if err != nil {
			http.Error(w, "Failed to parse template file", http.StatusInternalServerError)
			log.Printf("Template parse error: %v", err)
			return
		}

		if err := t.Execute(w, data); err != nil {
			log.Printf("Template execution error: %v", err)
		}
	})

	fmt.Println("Ledger Explorer running at http://localhost:8081 (or 8080 if run directly)")

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      nil, // uses http.DefaultServeMux
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

// extractLogsFromTiles walks the tile directory and rips JSON out of the binary files
func extractLogsFromTiles(tileDir string) []string {
	var extractedLogs []string

	root, err := os.OpenRoot(tileDir)
	if err != nil {
		log.Printf("Failed to open root directory %s: %v", tileDir, err)
		return extractedLogs
	}
	defer root.Close()

	fileSystem := root.FS()

	err = fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}

		data, readErr := fs.ReadFile(fileSystem, path)
		if readErr != nil {
			return nil
		}

		// Brute-force scan through the binary file looking for valid JSON blocks
		for i := 0; i < len(data); i++ {
			if data[i] == '{' {
				var obj map[string]interface{}
				dec := json.NewDecoder(bytes.NewReader(data[i:]))

				// If we successfully decode a JSON object from this byte position
				if err := dec.Decode(&obj); err == nil {
					if _, hasTime := obj["time"]; hasTime {
						prettyJSON, _ := json.MarshalIndent(obj, "", "  ")
						extractedLogs = append(extractedLogs, string(prettyJSON))
					}
					i += int(dec.InputOffset()) - 1
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Error encountered while walking tile directory: %v", err)
	}

	// Reverse the slice so the newest logs appear at the top of the UI
	for i, j := 0, len(extractedLogs)-1; i < j; i, j = i+1, j-1 {
		extractedLogs[i], extractedLogs[j] = extractedLogs[j], extractedLogs[i]
	}

	return extractedLogs
}
