//go:build memprobe

package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed memprobe.js
var memprobeJS string

var memprobeMu sync.Mutex

func init() {
	domReadyProbe = startMemprobe
}

func memprobeEnvInt(name string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return fallback
	}
	return v
}

func startMemprobe(ctx context.Context) {
	idle := memprobeEnvInt("PLAINSHELF_MEMPROBE_IDLE_SEC", 60)
	cycles := memprobeEnvInt("PLAINSHELF_MEMPROBE_CYCLES", 5)
	settle := memprobeEnvInt("PLAINSHELF_MEMPROBE_SETTLE_SEC", 15)
	script := fmt.Sprintf("%s\nwindow.__memprobe.run(%d, %d, %d);", memprobeJS, idle*1000, cycles, settle*1000)
	wailsruntime.WindowExecJS(ctx, script)
}

func (a *DesktopApp) MemprobeMark(phase string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	line := fmt.Sprintf("%d\t%s\t%d\t%d\t%d\n", time.Now().Unix(), phase, m.HeapInuse, m.HeapIdle-m.HeapReleased, m.Sys)

	memprobeMu.Lock()
	defer memprobeMu.Unlock()
	f, err := os.OpenFile(os.Getenv("PLAINSHELF_MEMPROBE_MARKS"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Println("memprobe:", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		log.Println("memprobe:", err)
	}
}
