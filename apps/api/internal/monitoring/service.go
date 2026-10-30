package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
)

type Sample struct {
	Slot, CollectedAt time.Time
	Status, Release   string
	Metrics           map[string]float64
}

type Period struct{ Start, End time.Time }

type DailyAggregate struct {
	Period           Period             `json:"period"`
	ReceivedSamples  int                `json:"receivedSamples"`
	ExpectedSamples  int                `json:"expectedSamples"`
	AccountsTotal    int64              `json:"accountsTotal"`
	NewRegistrations int64              `json:"newRegistrations"`
	SuccessfulLogins int64              `json:"successfulLogins"`
	UniqueLoggedIn   int64              `json:"uniqueLoggedIn"`
	FailedLogins     int64              `json:"failedLogins"`
	ActiveSessions   int64              `json:"activeSessions"`
	FailedSubsystems []string           `json:"failedSubsystems,omitempty"`
	Metrics          map[string]Summary `json:"metrics,omitempty"`
}

type Summary struct{ Average, P95, Maximum float64 }

type Store interface {
	SaveSample(context.Context, Sample) (bool, error)
	BuildDailyAggregate(context.Context, Period) (DailyAggregate, error)
	EnqueueDailyReport(context.Context, Period, DailyAggregate, []byte, int, time.Time) (int, error)
	ApplyAlertObservation(context.Context, AlertObservation, []byte, int) (bool, error)
}

type Collector interface {
	Collect(context.Context, time.Time) (Sample, error)
}

type PayloadEncryptor interface {
	Encrypt([]byte, string) ([]byte, int, error)
}

type AlertObservation struct {
	Type, Resource, State                  string
	At                                     time.Time
	Value, Threshold                       float64
	Peak                                   map[string]float64
	ImmediateActivation, ImmediateRecovery bool
}

type Service struct {
	store      Store
	collector  Collector
	cipher     PayloadEncryptor
	now        func() time.Time
	location   *time.Location
	reportHour int
	release    string
}

func NewService(store Store, collector Collector, cipher PayloadEncryptor, now func() time.Time, timezone string, reportHour int, release string) (*Service, error) {
	if store == nil || collector == nil || cipher == nil {
		return nil, fmt.Errorf("monitoring requires store, collector and notification cipher")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil || reportHour < 0 || reportHour > 23 {
		return nil, fmt.Errorf("invalid monitoring schedule")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, collector: collector, cipher: cipher, now: now, location: location, reportHour: reportHour, release: strings.TrimSpace(release)}, nil
}

func (service *Service) Tick(ctx context.Context) error {
	now := service.now().UTC()
	slot := now.Truncate(15 * time.Minute)
	sample, err := service.collector.Collect(ctx, slot)
	if err != nil {
		sample = Sample{Slot: slot, CollectedAt: now, Status: "failed", Metrics: map[string]float64{}}
	}
	sample.Slot, sample.CollectedAt, sample.Release = slot, now, service.release
	if sample.Status == "" {
		sample.Status = "complete"
	}
	if sample.Metrics == nil {
		sample.Metrics = map[string]float64{}
	}
	inserted, saveErr := service.store.SaveSample(ctx, sample)
	if saveErr != nil {
		return saveErr
	}
	if inserted {
		if err := service.evaluateAlerts(ctx, sample); err != nil {
			return err
		}
	}
	return service.enqueueDueDailyReports(ctx, now)
}

func (service *Service) enqueueDueDailyReports(ctx context.Context, now time.Time) error {
	localNow := now.In(service.location)
	todayDue := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), service.reportHour, 0, 0, 0, service.location)
	if localNow.Before(todayDue) {
		todayDue = todayDue.AddDate(0, 0, -1)
	}
	for daysAgo := 6; daysAgo >= 0; daysAgo-- {
		periodEndLocal := time.Date(todayDue.Year(), todayDue.Month(), todayDue.Day(), 0, 0, 0, 0, service.location).AddDate(0, 0, -daysAgo)
		period := Period{Start: periodEndLocal.AddDate(0, 0, -1).UTC(), End: periodEndLocal.UTC()}
		aggregate, err := service.store.BuildDailyAggregate(ctx, period)
		if err != nil {
			return err
		}
		message := service.dailyMessage(aggregate)
		payload, _ := json.Marshal(account.TelegramAuthMessage{Text: message})
		ciphertext, keyVersion, err := service.cipher.Encrypt(payload, "monitoring.daily_report")
		if err != nil {
			return err
		}
		if _, err := service.store.EnqueueDailyReport(ctx, period, aggregate, ciphertext, keyVersion, now); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) evaluateAlerts(ctx context.Context, sample Sample) error {
	type threshold struct {
		key, incident                          string
		activate, recover                      float64
		immediateActivation, immediateRecovery bool
	}
	thresholds := []threshold{
		{"cpu_busy_percent", "cpu.high", 85, 80, false, false},
		{"ram_working_set_percent", "ram.high", 85, 80, false, false},
		{"disk_used_percent", "disk.critical", 80, 75, true, true},
		{"io_wait_percent", "io_wait.high", 20, 10, false, false},
		{"postgres_connections_percent", "postgres.connections_high", 85, 70, false, false},
		{"containers_unhealthy", "container.unhealthy", 1, 0.5, true, false},
		{"container_unplanned_restarts", "container.restart", 1, 0.5, true, false},
		{"failed_login_spike", "auth.failed_login_spike", 1, 0.5, true, false},
	}
	for _, item := range thresholds {
		value, ok := sample.Metrics[item.key]
		if !ok {
			continue
		}
		state := "normal"
		thresholdValue := item.activate
		if value >= item.activate {
			state = "problem"
		} else if value < item.recover {
			state, thresholdValue = "recovered", item.recover
		}
		if state == "normal" {
			continue
		}
		immediateActivation := item.immediateActivation || value >= 95 && (item.incident == "cpu.high" || item.incident == "ram.high" || item.incident == "postgres.connections_high")
		observation := AlertObservation{Type: item.incident, Resource: "application", State: state, At: sample.Slot, Value: value, Threshold: thresholdValue, Peak: map[string]float64{"value": value}, ImmediateActivation: immediateActivation, ImmediateRecovery: item.immediateRecovery}
		text := fmt.Sprintf("arhDesign: %s\nСостояние: %s\nЗначение: %.1f%%, порог: %.1f%%\nВремя: %s (МСК)\nRunbook: %s", item.incident, state, value, thresholdValue, sample.Slot.In(service.location).Format("02.01.2006 15:04"), item.incident)
		payload, _ := json.Marshal(account.TelegramAuthMessage{Text: text})
		ciphertext, version, err := service.cipher.Encrypt(payload, "monitoring.incident")
		if err != nil {
			return err
		}
		if _, err := service.store.ApplyAlertObservation(ctx, observation, ciphertext, version); err != nil {
			return err
		}
	}
	if age, ok := sample.Metrics["backup_age_hours"]; ok {
		verified := sample.Metrics["backup_verified"] >= 1
		state, threshold := "normal", 26.0
		if !verified || age > threshold {
			state = "problem"
		} else {
			state = "recovered"
		}
		observation := AlertObservation{Type: "backup.stale", Resource: "offsite", State: state, At: sample.Slot, Value: age, Threshold: threshold, Peak: map[string]float64{"age_hours": age}, ImmediateActivation: true, ImmediateRecovery: true}
		payload, _ := json.Marshal(account.TelegramAuthMessage{Text: fmt.Sprintf("arhDesign: backup.stale\nСостояние: %s\nВозраст: %.1f ч\nВремя: %s (МСК)\nRunbook: backup.stale", state, age, sample.Slot.In(service.location).Format("02.01.2006 15:04"))})
		ciphertext, version, err := service.cipher.Encrypt(payload, "monitoring.incident")
		if err != nil {
			return err
		}
		if _, err := service.store.ApplyAlertObservation(ctx, observation, ciphertext, version); err != nil {
			return err
		}
	}
	collectorState := "recovered"
	if sample.Status != "complete" {
		collectorState = "problem"
	}
	payload, _ := json.Marshal(account.TelegramAuthMessage{Text: fmt.Sprintf("arhDesign: metrics.collector\nСостояние: %s\nВремя: %s (МСК)\nRunbook: metrics.collector", collectorState, sample.Slot.In(service.location).Format("02.01.2006 15:04"))})
	ciphertext, version, err := service.cipher.Encrypt(payload, "monitoring.incident")
	if err != nil {
		return err
	}
	_, err = service.store.ApplyAlertObservation(ctx, AlertObservation{Type: "metrics.collector", Resource: "application", State: collectorState, At: sample.Slot, Value: 1, Threshold: 2, Peak: map[string]float64{}}, ciphertext, version)
	if err != nil {
		return err
	}
	return nil
}

func (service *Service) dailyMessage(aggregate DailyAggregate) string {
	date := aggregate.Period.Start.In(service.location).Format("02.01.2006")
	prefix := ""
	if aggregate.ExpectedSamples == 0 || aggregate.ReceivedSamples*100 < aggregate.ExpectedSamples*90 || len(aggregate.FailedSubsystems) > 0 {
		prefix = "ДАННЫЕ НЕПОЛНЫЕ\n"
	}
	return fmt.Sprintf("%sСводка arhDesign за %s (МСК)\nДанные мониторинга: %d/%d интервалов\nСистема: CPU p95 %s; RAM p95 %s; диск max %s; backup age %s\nПользователи: всего %d; регистрации %d\nВходы: успешные %d; уникальные %d; неуспешные %d; активные сессии %d",
		prefix, date, aggregate.ReceivedSamples, aggregate.ExpectedSamples,
		metricValue(aggregate.Metrics, "cpu_busy_percent", "p95"), metricValue(aggregate.Metrics, "ram_working_set_percent", "p95"),
		metricValue(aggregate.Metrics, "disk_used_percent", "max"), metricValue(aggregate.Metrics, "backup_age_hours", "max"),
		aggregate.AccountsTotal, aggregate.NewRegistrations,
		aggregate.SuccessfulLogins, aggregate.UniqueLoggedIn, aggregate.FailedLogins, aggregate.ActiveSessions)
}

func metricValue(metrics map[string]Summary, key, field string) string {
	value, ok := metrics[key]
	if !ok {
		return "n/a"
	}
	if field == "max" {
		return fmt.Sprintf("%.1f", value.Maximum)
	}
	return fmt.Sprintf("%.1f", value.P95)
}

type RuntimeCollector struct {
	backupDirectory string
	mu              sync.Mutex
	previousCPUUse  int64
	previousCPUAt   time.Time
}

func NewRuntimeCollector(backupDirectory string) *RuntimeCollector {
	return &RuntimeCollector{backupDirectory: strings.TrimSpace(backupDirectory)}
}

func (collector *RuntimeCollector) Collect(_ context.Context, slot time.Time) (Sample, error) {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	metrics := map[string]float64{
		"go_goroutines": float64(runtime.NumGoroutine()), "go_heap_bytes": float64(memory.HeapAlloc),
	}
	complete := true
	if current, maximum, ok := cgroupMemory(); ok {
		metrics["ram_working_set_bytes"] = float64(current)
		metrics["ram_working_set_percent"] = float64(current) * 100 / float64(maximum)
	} else {
		complete = false
	}
	if used, percent, ok := filesystemUsage("/"); ok {
		metrics["disk_used_bytes"] = float64(used)
		metrics["disk_used_percent"] = percent
	} else {
		complete = false
	}
	if usage, capacity, ok := cgroupCPU(); ok {
		collector.mu.Lock()
		if collector.previousCPUUse > 0 && slot.After(collector.previousCPUAt) {
			elapsedMicros := slot.Sub(collector.previousCPUAt).Microseconds()
			metrics["cpu_busy_percent"] = float64(usage-collector.previousCPUUse) * 100 / float64(elapsedMicros) / capacity
		} else {
			complete = false
		}
		collector.previousCPUUse, collector.previousCPUAt = usage, slot
		collector.mu.Unlock()
	} else {
		complete = false
	}
	if collector.backupDirectory != "" {
		if age, ok := verifiedBackupAge(collector.backupDirectory, slot); ok {
			metrics["backup_age_hours"], metrics["backup_verified"] = age, 1
		} else {
			metrics["backup_age_hours"], metrics["backup_verified"] = 1_000_000, 0
			complete = false
		}
	}
	status := "complete"
	if !complete {
		status = "partial"
	}
	return Sample{Slot: slot, Status: status, Metrics: metrics}, nil
}

func UntilNextQuarterHour(now time.Time) time.Duration {
	next := now.Truncate(15 * time.Minute).Add(15 * time.Minute)
	return next.Sub(now)
}

func cgroupMemory() (int64, int64, bool) {
	currentText, currentErr := os.ReadFile("/sys/fs/cgroup/memory.current")
	maximumText, maximumErr := os.ReadFile("/sys/fs/cgroup/memory.max")
	if currentErr != nil || maximumErr != nil || strings.TrimSpace(string(maximumText)) == "max" {
		return 0, 0, false
	}
	current, currentErr := strconv.ParseInt(strings.TrimSpace(string(currentText)), 10, 64)
	maximum, maximumErr := strconv.ParseInt(strings.TrimSpace(string(maximumText)), 10, 64)
	return current, maximum, currentErr == nil && maximumErr == nil && maximum > 0
}

func cgroupCPU() (int64, float64, bool) {
	stat, statErr := os.ReadFile("/sys/fs/cgroup/cpu.stat")
	maximum, maximumErr := os.ReadFile("/sys/fs/cgroup/cpu.max")
	if statErr != nil || maximumErr != nil {
		return 0, 0, false
	}
	var usage int64
	for _, line := range strings.Split(string(stat), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "usage_usec" {
			usage, _ = strconv.ParseInt(fields[1], 10, 64)
		}
	}
	fields := strings.Fields(string(maximum))
	if usage <= 0 || len(fields) != 2 {
		return 0, 0, false
	}
	if fields[0] == "max" {
		return usage, float64(runtime.NumCPU()), true
	}
	quota, quotaErr := strconv.ParseFloat(fields[0], 64)
	period, periodErr := strconv.ParseFloat(fields[1], 64)
	if quotaErr != nil || periodErr != nil || quota <= 0 || period <= 0 {
		return 0, 0, false
	}
	return usage, quota / period, true
}

func filesystemUsage(path string) (uint64, float64, bool) {
	var value syscall.Statfs_t
	if err := syscall.Statfs(path, &value); err != nil || value.Blocks == 0 {
		return 0, 0, false
	}
	usedBlocks := value.Blocks - value.Bavail
	return usedBlocks * uint64(value.Bsize), float64(usedBlocks) * 100 / float64(value.Blocks), true
}

func verifiedBackupAge(directory string, now time.Time) (float64, bool) {
	value, err := os.ReadFile(directory + "/latest-offsite-success")
	if err != nil {
		return 0, false
	}
	completedAt, err := time.Parse("20060102T150405Z", strings.TrimSpace(string(value)))
	if err != nil || completedAt.After(now.Add(5*time.Minute)) {
		return 0, false
	}
	return now.Sub(completedAt).Hours(), true
}

func Summarize(values []float64) Summary {
	if len(values) == 0 {
		return Summary{}
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	var sum float64
	for _, value := range ordered {
		sum += value
	}
	index := (95*len(ordered)+99)/100 - 1
	if index < 0 {
		index = 0
	}
	return Summary{Average: sum / float64(len(ordered)), P95: ordered[index], Maximum: ordered[len(ordered)-1]}
}
