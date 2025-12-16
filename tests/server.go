package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

var totalBytesSent int64
var startTime = time.Now()

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
	
	totalBytesSent += int64(n)
	duration := time.Since(start)
	throughput := float64(n) / duration.Seconds() / 1024 / 1024
	
	log.Printf("Sent %d bytes in %v (%.2f MB/s)", n, duration, throughput)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	elapsed := time.Since(startTime)
	avgThroughput := float64(totalBytesSent) / elapsed.Seconds() / 1024 / 1024
	
	fmt.Fprintf(w, "Total bytes sent: %d\n", totalBytesSent)
	fmt.Fprintf(w, "Uptime: %v\n", elapsed)
	fmt.Fprintf(w, "Average throughput: %.2f MB/s\n", avgThroughput)
}

func main() {
	http.HandleFunc("/test", testHandler)
	http.HandleFunc("/metrics", metricsHandler)
	
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}