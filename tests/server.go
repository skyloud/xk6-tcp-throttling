package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type ServerResult struct {
	PayloadMB      string `json:"payload_mb"`
	DurationS      string `json:"duration_s"`
	ThroughputMBps string `json:"throughput_mbps"`
	Timestamp      string `json:"timestamp"`
}

var totalBytesSent int64
var startTime = time.Now()
var serverResults []ServerResult
var resultsMutex sync.Mutex

func testHandler(w http.ResponseWriter, r *http.Request) {
	size := 1024 * 1024 // 1MB par défaut
	if s := r.URL.Query().Get("size"); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil {
			size = parsed
		}
	}

	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(size))
	
	start := time.Now()
	n, err := w.Write(data)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	
	// Forcer le flush
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	
	// Accéder à la connexion TCP sous-jacente pour attendre les ACKs
	hijacker, ok := w.(http.Hijacker)
	if ok {
		conn, _, err := hijacker.Hijack()
		if err == nil {
			// Attendre que toutes les données soient ACKées
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				// CloseWrite envoie FIN et attend que le buffer soit vidé
				tcpConn.CloseWrite()
				// Attendre la réponse du client (FIN)
				buf := make([]byte, 1)
				conn.Read(buf)
			}
			conn.Close()
			
			duration := time.Since(start)
			throughput := float64(n) / duration.Seconds() / 1024 / 1024
			log.Printf("Sent %d bytes in %v (%.2f MB/s) [TCP measured]", n, duration, throughput)
			
			resultsMutex.Lock()
			serverResults = append(serverResults, ServerResult{
				PayloadMB:      fmt.Sprintf("%.2f", float64(n)/1024/1024),
				DurationS:      fmt.Sprintf("%.2f", duration.Seconds()),
				ThroughputMBps: fmt.Sprintf("%.2f", throughput),
				Timestamp:      time.Now().Format(time.RFC3339),
			})
			resultsMutex.Unlock()
			
			totalBytesSent += int64(n)
			return
		}
	}
	
	// Fallback si hijack ne fonctionne pas
	totalBytesSent += int64(n)
	duration := time.Since(start)
	throughput := float64(n) / duration.Seconds() / 1024 / 1024
	log.Printf("Sent %d bytes in %v (%.2f MB/s) [buffer only]", n, duration, throughput)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	elapsed := time.Since(startTime)
	avgThroughput := float64(totalBytesSent) / elapsed.Seconds() / 1024 / 1024
	
	fmt.Fprintf(w, "Total bytes sent: %d\n", totalBytesSent)
	fmt.Fprintf(w, "Uptime: %v\n", elapsed)
	fmt.Fprintf(w, "Average throughput: %.2f MB/s\n", avgThroughput)
}

func saveResultsHandler(w http.ResponseWriter, r *http.Request) {
	resultsMutex.Lock()
	defer resultsMutex.Unlock()
	
	data, err := json.MarshalIndent(serverResults, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	
	err = os.WriteFile("/results/server-results.json", data, 0644)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	
	w.WriteHeader(200)
	fmt.Fprintf(w, "Results saved")
}

func main() {
	http.HandleFunc("/test", testHandler)
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/save-results", saveResultsHandler)
	
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}