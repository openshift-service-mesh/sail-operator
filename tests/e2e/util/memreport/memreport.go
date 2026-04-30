//go:build e2e

// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memreport

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	. "github.com/onsi/ginkgo/v2"
)

var (
	reporter     *MemoryReporter
	reporterOnce sync.Once
)

type MemorySnapshot struct {
	Timestamp      time.Time
	SuiteName      string
	SpecName       string
	Phase          string
	AllocMB        float64
	TotalAllocMB   float64
	SysMB          float64
	HeapAllocMB    float64
	HeapSysMB      float64
	HeapIdleMB     float64
	HeapInuseMB    float64
	HeapReleasedMB float64
	HeapObjects    uint64
	NumGC          uint32
	NumGoroutine   int
	// Storage metrics
	DiskTotalGB     float64
	DiskUsedGB      float64
	DiskAvailGB     float64
	DiskUsedPercent float64
}

type MemoryReporter struct {
	mu          sync.Mutex
	snapshots   []MemorySnapshot
	suiteName   string
	startTime   time.Time
	peakHeapMB  float64
	artifactDir string
	printToStdout bool
}

func GetReporter() *MemoryReporter {
	reporterOnce.Do(func() {
		reporter = &MemoryReporter{
			snapshots:   make([]MemorySnapshot, 0),
			artifactDir: env.Get("ARTIFACTS", "/tmp/artifacts"),
		}
	})
	return reporter
}

func (r *MemoryReporter) SetSuiteName(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suiteName = name
	r.startTime = time.Now()
}

func (r *MemoryReporter) SetPrintToStdout(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.printToStdout = enabled
}

func getDiskStats(path string) (totalGB, usedGB, availGB, usedPercent float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, 0
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - (stat.Bfree * uint64(stat.Bsize))

	totalGB = float64(totalBytes) / 1024 / 1024 / 1024
	usedGB = float64(usedBytes) / 1024 / 1024 / 1024
	availGB = float64(availBytes) / 1024 / 1024 / 1024
	if totalGB > 0 {
		usedPercent = (usedGB / totalGB) * 100
	}
	return
}

func (r *MemoryReporter) TakeSnapshot(phase string) MemorySnapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	specName := ""
	if CurrentSpecReport().LeafNodeText != "" {
		specName = CurrentSpecReport().LeafNodeText
	}

	// Get disk stats for the artifact directory (or root if not set)
	diskPath := r.artifactDir
	if diskPath == "" {
		diskPath = "/"
	}
	diskTotal, diskUsed, diskAvail, diskPercent := getDiskStats(diskPath)

	snapshot := MemorySnapshot{
		Timestamp:       time.Now(),
		SuiteName:       r.suiteName,
		SpecName:        specName,
		Phase:           phase,
		AllocMB:         float64(m.Alloc) / 1024 / 1024,
		TotalAllocMB:    float64(m.TotalAlloc) / 1024 / 1024,
		SysMB:           float64(m.Sys) / 1024 / 1024,
		HeapAllocMB:     float64(m.HeapAlloc) / 1024 / 1024,
		HeapSysMB:       float64(m.HeapSys) / 1024 / 1024,
		HeapIdleMB:      float64(m.HeapIdle) / 1024 / 1024,
		HeapInuseMB:     float64(m.HeapInuse) / 1024 / 1024,
		HeapReleasedMB:  float64(m.HeapReleased) / 1024 / 1024,
		HeapObjects:     m.HeapObjects,
		NumGC:           m.NumGC,
		NumGoroutine:    runtime.NumGoroutine(),
		DiskTotalGB:     diskTotal,
		DiskUsedGB:      diskUsed,
		DiskAvailGB:     diskAvail,
		DiskUsedPercent: diskPercent,
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshots = append(r.snapshots, snapshot)
	if snapshot.HeapAllocMB > r.peakHeapMB {
		r.peakHeapMB = snapshot.HeapAllocMB
	}

	return snapshot
}

func (r *MemoryReporter) LogSnapshot(snapshot MemorySnapshot) {
	GinkgoWriter.Printf("\n=== Memory Report [%s - %s] ===\n", snapshot.Phase, snapshot.SuiteName)
	if snapshot.SpecName != "" {
		GinkgoWriter.Printf("Spec: %s\n", snapshot.SpecName)
	}
	GinkgoWriter.Printf("Timestamp: %s\n", snapshot.Timestamp.Format(time.RFC3339))
	GinkgoWriter.Printf("Heap Alloc:    %.2f MB\n", snapshot.HeapAllocMB)
	GinkgoWriter.Printf("Heap Sys:      %.2f MB\n", snapshot.HeapSysMB)
	GinkgoWriter.Printf("Heap Inuse:    %.2f MB\n", snapshot.HeapInuseMB)
	GinkgoWriter.Printf("Heap Idle:     %.2f MB\n", snapshot.HeapIdleMB)
	GinkgoWriter.Printf("Heap Released: %.2f MB\n", snapshot.HeapReleasedMB)
	GinkgoWriter.Printf("Heap Objects:  %d\n", snapshot.HeapObjects)
	GinkgoWriter.Printf("Total Alloc:   %.2f MB (cumulative)\n", snapshot.TotalAllocMB)
	GinkgoWriter.Printf("Sys Memory:    %.2f MB\n", snapshot.SysMB)
	GinkgoWriter.Printf("Goroutines:    %d\n", snapshot.NumGoroutine)
	GinkgoWriter.Printf("GC Cycles:     %d\n", snapshot.NumGC)
	GinkgoWriter.Printf("--- Storage ---\n")
	GinkgoWriter.Printf("Disk Total:    %.2f GB\n", snapshot.DiskTotalGB)
	GinkgoWriter.Printf("Disk Used:     %.2f GB (%.1f%%)\n", snapshot.DiskUsedGB, snapshot.DiskUsedPercent)
	GinkgoWriter.Printf("Disk Avail:    %.2f GB\n", snapshot.DiskAvailGB)
	GinkgoWriter.Println("================================\n")
}

func (r *MemoryReporter) Report(phase string) {
	snapshot := r.TakeSnapshot(phase)
	r.LogSnapshot(snapshot)
}

func (r *MemoryReporter) ReportAndForceGC(phase string) {
	r.Report(phase + " (before GC)")
	runtime.GC()
	r.Report(phase + " (after GC)")
}

func (r *MemoryReporter) WriteSummary() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.snapshots) == 0 {
		return
	}

	if r.printToStdout {
		r.printSummaryToStdout()
		return
	}

	memDir := filepath.Join(r.artifactDir, "memory-reports")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		GinkgoWriter.Printf("Warning: failed to create memory report directory: %v\n", err)
		return
	}

	fileName := filepath.Join(memDir, fmt.Sprintf("memory-%s-%s.csv",
		r.suiteName,
		time.Now().Format("20060102-150405")))

	file, err := os.Create(fileName)
	if err != nil {
		GinkgoWriter.Printf("Warning: failed to create memory report file: %v\n", err)
		return
	}
	defer file.Close()

	// Write CSV header
	fmt.Fprintln(file, "Timestamp,Suite,Spec,Phase,HeapAllocMB,HeapSysMB,HeapInuseMB,HeapIdleMB,HeapReleasedMB,HeapObjects,TotalAllocMB,SysMB,Goroutines,GCCycles,DiskTotalGB,DiskUsedGB,DiskAvailGB,DiskUsedPercent")

	for _, s := range r.snapshots {
		fmt.Fprintf(file, "%s,%s,%s,%s,%.2f,%.2f,%.2f,%.2f,%.2f,%d,%.2f,%.2f,%d,%d,%.2f,%.2f,%.2f,%.1f\n",
			s.Timestamp.Format(time.RFC3339),
			s.SuiteName,
			s.SpecName,
			s.Phase,
			s.HeapAllocMB,
			s.HeapSysMB,
			s.HeapInuseMB,
			s.HeapIdleMB,
			s.HeapReleasedMB,
			s.HeapObjects,
			s.TotalAllocMB,
			s.SysMB,
			s.NumGoroutine,
			s.NumGC,
			s.DiskTotalGB,
			s.DiskUsedGB,
			s.DiskAvailGB,
			s.DiskUsedPercent,
		)
	}

	GinkgoWriter.Printf("\n=== Memory Summary for %s ===\n", r.suiteName)
	GinkgoWriter.Printf("Duration: %s\n", time.Since(r.startTime).Round(time.Second))
	GinkgoWriter.Printf("Snapshots: %d\n", len(r.snapshots))
	GinkgoWriter.Printf("Peak Heap: %.2f MB\n", r.peakHeapMB)

	if len(r.snapshots) >= 2 {
		first := r.snapshots[0]
		last := r.snapshots[len(r.snapshots)-1]
		GinkgoWriter.Printf("Heap Growth: %.2f MB -> %.2f MB (delta: %.2f MB)\n",
			first.HeapAllocMB, last.HeapAllocMB, last.HeapAllocMB-first.HeapAllocMB)
		GinkgoWriter.Printf("Total Alloc Growth: %.2f MB\n", last.TotalAllocMB-first.TotalAllocMB)
		GinkgoWriter.Printf("Goroutine Growth: %d -> %d\n", first.NumGoroutine, last.NumGoroutine)
	}

	GinkgoWriter.Printf("Report written to: %s\n", fileName)
	GinkgoWriter.Println("================================\n")
}

func (r *MemoryReporter) printSummaryToStdout() {
	GinkgoWriter.Println()
	GinkgoWriter.Printf("=== Memory Report for %s ===\n", r.suiteName)
	GinkgoWriter.Println("Timestamp,Suite,Spec,Phase,HeapAllocMB,HeapSysMB,HeapInuseMB,HeapIdleMB,HeapReleasedMB,HeapObjects,TotalAllocMB,SysMB,Goroutines,GCCycles,DiskTotalGB,DiskUsedGB,DiskAvailGB,DiskUsedPercent")

	for _, s := range r.snapshots {
		GinkgoWriter.Printf("%s,%s,%s,%s,%.2f,%.2f,%.2f,%.2f,%.2f,%d,%.2f,%.2f,%d,%d,%.2f,%.2f,%.2f,%.1f\n",
			s.Timestamp.Format(time.RFC3339),
			s.SuiteName,
			s.SpecName,
			s.Phase,
			s.HeapAllocMB,
			s.HeapSysMB,
			s.HeapInuseMB,
			s.HeapIdleMB,
			s.HeapReleasedMB,
			s.HeapObjects,
			s.TotalAllocMB,
			s.SysMB,
			s.NumGoroutine,
			s.NumGC,
			s.DiskTotalGB,
			s.DiskUsedGB,
			s.DiskAvailGB,
			s.DiskUsedPercent,
		)
	}

	GinkgoWriter.Printf("\n=== Memory Summary for %s ===\n", r.suiteName)
	GinkgoWriter.Printf("Duration: %s\n", time.Since(r.startTime).Round(time.Second))
	GinkgoWriter.Printf("Snapshots: %d\n", len(r.snapshots))
	GinkgoWriter.Printf("Peak Heap: %.2f MB\n", r.peakHeapMB)

	if len(r.snapshots) >= 2 {
		first := r.snapshots[0]
		last := r.snapshots[len(r.snapshots)-1]
		GinkgoWriter.Printf("Heap Growth: %.2f MB -> %.2f MB (delta: %.2f MB)\n",
			first.HeapAllocMB, last.HeapAllocMB, last.HeapAllocMB-first.HeapAllocMB)
		GinkgoWriter.Printf("Total Alloc Growth: %.2f MB\n", last.TotalAllocMB-first.TotalAllocMB)
		GinkgoWriter.Printf("Goroutine Growth: %d -> %d\n", first.NumGoroutine, last.NumGoroutine)
	}
	GinkgoWriter.Println("================================")
}

func SetupMemoryReporting(suiteName string) {
	setupMemoryReportingInternal(suiteName, false)
}

func SetupMemoryReportingWithPrint(suiteName string) {
	setupMemoryReportingInternal(suiteName, true)
}

func setupMemoryReportingInternal(suiteName string, printToStdout bool) {
	reporter := GetReporter()
	reporter.SetSuiteName(suiteName)
	reporter.SetPrintToStdout(printToStdout)

	BeforeSuite(func() {
		reporter.Report("BeforeSuite")
	})

	AfterSuite(func() {
		reporter.ReportAndForceGC("AfterSuite")
		reporter.WriteSummary()
	})

	BeforeEach(func() {
		reporter.Report("BeforeEach")
	})

	AfterEach(func() {
		reporter.Report("AfterEach")
		if CurrentSpecReport().Failed() {
			reporter.ReportAndForceGC("AfterEach-Failed")
		}
	})
}
