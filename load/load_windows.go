// +build windows

package load

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/internal/common"
)

var (
	loadErr              error
	loadAvg1M            float64 = 0.0
	loadAvg5M            float64 = 0.0
	loadAvg15M           float64 = 0.0
	loadAvgMutex         sync.RWMutex
	loadAvgGoroutineOnce sync.Once
)

// loadAvgGoroutine updates avg data by fetching current load by interval
// TODO instead of this goroutine, we can register a Win32 counter just as psutil does
// see https://psutil.readthedocs.io/en/latest/#psutil.getloadavg
// code https://github.com/giampaolo/psutil/blob/8415355c8badc9c94418b19bdf26e622f06f0cce/psutil/arch/windows/wmi.c
func loadAvgGoroutine() {
	var (
		samplingFrequency time.Duration = 5 * time.Second
		loadAvgFactor1M   float64       = 1 / math.Exp(samplingFrequency.Seconds()/time.Minute.Seconds())
		loadAvgFactor5M   float64       = 1 / math.Exp(samplingFrequency.Seconds()/(5*time.Minute).Seconds())
		loadAvgFactor15M  float64       = 1 / math.Exp(samplingFrequency.Seconds()/(15*time.Minute).Seconds())
		currentLoad       float64
	)

	counter, err := common.ProcessorQueueLengthCounter()
	loadErr = err
	if err != nil || counter == nil {
		loadAvgMutex.Unlock()
		log.Println("unexpected processor queue length counter error, please file an issue on github")
		return
	}

	tick := time.NewTicker(samplingFrequency).C
	for {
		currentLoad, loadErr = counter.GetValue()
		if loadErr != nil {
			goto SKIP
		}
		// comment following block if you want load to be 0 as long as process queue is zero
		{
			if currentLoad == 0.0 {
				percent, err := cpu.Percent(0, false)
				if err == nil {
					currentLoad = percent[0] / 100
					// load averages are also given some amount of the currentLoad
					// maybe they shouldnt?
					if loadAvg1M == 0 {
						loadAvg1M = currentLoad
					}
					if loadAvg5M == 0 {
						loadAvg5M = currentLoad / 2
					}
					if loadAvg15M == 0 {
						loadAvg15M = currentLoad / 3
					}
				}
			}
		}
		loadAvg1M = loadAvg1M*loadAvgFactor1M + currentLoad*(1-loadAvgFactor1M)
		loadAvg5M = loadAvg5M*loadAvgFactor5M + currentLoad*(1-loadAvgFactor5M)
		loadAvg15M = loadAvg15M*loadAvgFactor15M + currentLoad*(1-loadAvgFactor15M)

	SKIP:
		loadAvgMutex.Unlock()
		<-tick
		loadAvgMutex.Lock()
	}
}

func Avg() (*AvgStat, error) {
	return AvgWithContext(context.Background())
}

func AvgWithContext(ctx context.Context) (*AvgStat, error) {
	loadAvgGoroutineOnce.Do(func() {
		loadAvgMutex.Lock()
		go loadAvgGoroutine()
	})
	loadAvgMutex.RLock()
	defer loadAvgMutex.RUnlock()
	ret := AvgStat{
		Load1:  loadAvg1M,
		Load5:  loadAvg5M,
		Load15: loadAvg15M,
	}

	return &ret, loadErr
}

func Misc() (*MiscStat, error) {
	return MiscWithContext(context.Background())
}

func MiscWithContext(ctx context.Context) (*MiscStat, error) {
	ret := MiscStat{}

	return &ret, common.ErrNotImplementedError
}
