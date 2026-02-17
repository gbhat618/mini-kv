package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type BenchmarkResult struct {
	Name         string  `json:"name"`
	Iterations   int     `json:"iterations"`
	DurationMs   float64 `json:"duration_ms"`
	OpsPerSec    float64 `json:"ops_per_sec"`
	AvgLatencyUs float64 `json:"avg_latency_us"`
	MinLatencyUs float64 `json:"min_latency_us"`
	MaxLatencyUs float64 `json:"max_latency_us"`
	Percentile95 float64 `json:"p95_latency_us"`
	Percentile99 float64 `json:"p99_latency_us"`
	Errors       int     `json:"errors"`
	Concurrency  int     `json:"concurrency"`
}

func connectBenchmark(host string, port int) (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	return net.Dial("tcp", addr)
}

func sendCommandBenchmark(conn net.Conn, cmd string) string {
	fmt.Fprintf(conn, "%s\n", cmd)
	reader := bufio.NewReader(conn)
	line, _ := reader.ReadString('\n')
	return line
}

func runBenchmark(host string, port int, name string, concurrency int, iterations int, setup func(), fn func(net.Conn)) BenchmarkResult {
	var totalErrors int64
	var totalDuration int64
	var minLatency int64 = ^int64(0)
	var maxLatency int64
	var mu sync.Mutex
	var latencies []int64

	setup()

	var wg sync.WaitGroup
	perThread := iterations / concurrency
	if perThread == 0 {
		perThread = 1
	}

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := connectBenchmark(host, port)
			if err != nil {
				atomic.AddInt64(&totalErrors, 1)
				return
			}
			defer conn.Close()

			localLatencies := make([]int64, 0, perThread)
			for j := 0; j < perThread; j++ {
				t0 := time.Now()
				fn(conn)
				latency := time.Since(t0).Nanoseconds()
				localLatencies = append(localLatencies, latency)
				atomic.AddInt64(&totalDuration, latency)
			}
			mu.Lock()
			latencies = append(latencies, localLatencies...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	duration := time.Since(start)
	opsPerSec := float64(iterations) / duration.Seconds()
	avgLatency := time.Duration(atomic.LoadInt64(&totalDuration)) / time.Duration(iterations)

	if minLatency == ^int64(0) {
		minLatency = 0
	}

	for _, l := range latencies {
		if l < minLatency {
			minLatency = l
		}
		if l > maxLatency {
			maxLatency = l
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var p95, p99 time.Duration
	if len(latencies) > 0 {
		p95Idx := len(latencies) * 95 / 100
		p99Idx := len(latencies) * 99 / 100
		if p95Idx >= len(latencies) {
			p95Idx = len(latencies) - 1
		}
		if p99Idx >= len(latencies) {
			p99Idx = len(latencies) - 1
		}
		p95 = time.Duration(latencies[p95Idx])
		p99 = time.Duration(latencies[p99Idx])
	}

	return BenchmarkResult{
		Name:         name,
		Iterations:   iterations,
		DurationMs:   float64(duration.Milliseconds()),
		OpsPerSec:    opsPerSec,
		AvgLatencyUs: float64(avgLatency.Microseconds()),
		MinLatencyUs: float64(time.Duration(minLatency).Microseconds()),
		MaxLatencyUs: float64(time.Duration(maxLatency).Microseconds()),
		Percentile95: float64(p95.Microseconds()),
		Percentile99: float64(p99.Microseconds()),
		Errors:       int(totalErrors),
		Concurrency:  concurrency,
	}
}

func main() {
	host := "localhost"
	port := 6379
	iterations := 5000
	concurrency := 10

	if len(os.Args) > 1 {
		host = os.Args[1]
	}
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &port)
	}
	if len(os.Args) > 3 {
		fmt.Sscanf(os.Args[3], "%d", &iterations)
	}
	if len(os.Args) > 4 {
		fmt.Sscanf(os.Args[4], "%d", &concurrency)
	}

	conn, err := connectBenchmark(host, port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	conn.Close()

	fmt.Printf("Running benchmarks: %d iterations, %d concurrency\n\n", iterations, concurrency)

	var results []BenchmarkResult

	fmt.Println("=== PING ===")
	result := runBenchmark(host, port, "PING", concurrency, iterations,
		func() {},
		func(c net.Conn) { sendCommandBenchmark(c, "PING") })
	results = append(results, result)
	printResult(result)

	fmt.Println("\n=== SET ===")
	result = runBenchmark(host, port, "SET", concurrency, iterations,
		func() {
			c, _ := connectBenchmark(host, port)
			sendCommandBenchmark(c, "FLUSHDB")
			c.Close()
		},
		func(c net.Conn) { sendCommandBenchmark(c, "SET benchmark_key value") })
	results = append(results, result)
	printResult(result)

	fmt.Println("\n=== GET (with 1000 keys) ===")
	result = runBenchmark(host, port, "GET", concurrency, iterations,
		func() {
			c, _ := connectBenchmark(host, port)
			sendCommandBenchmark(c, "FLUSHDB")
			for i := 0; i < 1000; i++ {
				sendCommandBenchmark(c, fmt.Sprintf("SET key%d value%d", i, i))
			}
			c.Close()
		},
		func(c net.Conn) { sendCommandBenchmark(c, "GET key0") })
	results = append(results, result)
	printResult(result)

	fmt.Println("\n=== MIXED (GET/SET) ===")
	result = runBenchmark(host, port, "MIXED", concurrency, iterations,
		func() {
			c, _ := connectBenchmark(host, port)
			sendCommandBenchmark(c, "FLUSHDB")
			for i := 0; i < 1000; i++ {
				sendCommandBenchmark(c, fmt.Sprintf("SET key%d value%d", i, i))
			}
			c.Close()
		},
		func(c net.Conn) {
			sendCommandBenchmark(c, "GET key0")
			sendCommandBenchmark(c, "SET temp test")
		})
	results = append(results, result)
	printResult(result)

	jsonData, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile("benchmark_report.json", jsonData, 0644)
	fmt.Println("\nDetailed report saved to benchmark_report.json")
}

func printResult(r BenchmarkResult) {
	fmt.Printf("  Operations/sec: %.2f\n", r.OpsPerSec)
	fmt.Printf("  Avg latency: %.2f us\n", r.AvgLatencyUs)
	fmt.Printf("  P95 latency: %.2f us\n", r.Percentile95)
	fmt.Printf("  P99 latency: %.2f us\n", r.Percentile99)
	fmt.Printf("  Min/Max: %.2f / %.2f us\n", r.MinLatencyUs, r.MaxLatencyUs)
}
