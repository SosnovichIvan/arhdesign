package monitoring

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
)

type fakeStore struct {
	samples      []Sample
	inserted     bool
	periods      []Period
	alerts       []AlertObservation
	aggregate    DailyAggregate
	saveErr      error
	aggregateErr error
	enqueueErr   error
	alertErr     error
}

func (store *fakeStore) SaveSample(_ context.Context, sample Sample) (bool, error) {
	store.samples = append(store.samples, sample)
	if store.saveErr != nil {
		return false, store.saveErr
	}
	inserted := store.inserted
	store.inserted = false
	return inserted, nil
}
func (store *fakeStore) BuildDailyAggregate(_ context.Context, period Period) (DailyAggregate, error) {
	if store.aggregateErr != nil {
		return DailyAggregate{}, store.aggregateErr
	}
	if store.aggregate.ExpectedSamples > 0 {
		value := store.aggregate
		value.Period = period
		return value, nil
	}
	return DailyAggregate{Period: period, ExpectedSamples: 96, ReceivedSamples: 96}, nil
}
func (store *fakeStore) EnqueueDailyReport(_ context.Context, period Period, _ DailyAggregate, ciphertext []byte, version int, _ time.Time) (int, error) {
	if store.enqueueErr != nil {
		return 0, store.enqueueErr
	}
	if len(ciphertext) == 0 || version != 1 {
		return 0, errors.New("invalid encrypted report")
	}
	store.periods = append(store.periods, period)
	return 1, nil
}
func (store *fakeStore) ApplyAlertObservation(_ context.Context, observation AlertObservation, ciphertext []byte, version int) (bool, error) {
	if store.alertErr != nil {
		return false, store.alertErr
	}
	if len(ciphertext) == 0 || version != 1 {
		return false, errors.New("invalid encrypted alert")
	}
	store.alerts = append(store.alerts, observation)
	return true, nil
}

type fakeCollector struct {
	sample Sample
	err    error
}

type failingCipher struct{}

func (failingCipher) Encrypt([]byte, string) ([]byte, int, error) {
	return nil, 0, errors.New("encryption failed")
}

func (collector fakeCollector) Collect(context.Context, time.Time) (Sample, error) {
	return collector.sample, collector.err
}

func monitoringServiceForTest(t *testing.T, store Store, collector Collector, now time.Time) *Service {
	t.Helper()
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, collector, cipher, func() time.Time { return now }, "Europe/Moscow", 9, "test")
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestTickUsesQuarterHourSlotCatchesUpSevenPeriodsAndAvoidsDuplicateAlertEvaluation(t *testing.T) {
	now := time.Date(2026, 10, 2, 7, 7, 0, 0, time.UTC) // 10:07 MSK
	store := &fakeStore{inserted: true}
	collector := fakeCollector{sample: Sample{Status: "complete", Metrics: map[string]float64{"disk_used_percent": 82}}}
	service := monitoringServiceForTest(t, store, collector, now)
	if err := service.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := store.samples[0].Slot; !got.Equal(time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("slot=%v", got)
	}
	if len(store.periods) != 7 || store.periods[6].End.In(service.location).Hour() != 0 {
		t.Fatalf("periods=%#v", store.periods)
	}
	if len(store.alerts) != 2 || store.alerts[0].Type != "disk.critical" || store.alerts[1].Type != "metrics.collector" {
		t.Fatalf("alerts=%#v", store.alerts)
	}
	if err := service.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.alerts) != 2 {
		t.Fatalf("duplicate slot evaluated alerts: %#v", store.alerts)
	}
}

func TestTickStoresFailedCollectorWithoutSensitiveDetail(t *testing.T) {
	store := &fakeStore{inserted: true}
	service := monitoringServiceForTest(t, store, fakeCollector{err: errors.New("secret raw collector error")}, time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC))
	if err := service.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.samples) != 1 || store.samples[0].Status != "failed" || len(store.samples[0].Metrics) != 0 {
		t.Fatalf("sample=%#v", store.samples)
	}
}

func TestSummarizeUsesNearestRankP95(t *testing.T) {
	values := make([]float64, 100)
	for index := range values {
		values[index] = float64(index + 1)
	}
	summary := Summarize(values)
	if summary.Average != 50.5 || summary.P95 != 95 || summary.Maximum != 100 {
		t.Fatalf("summary=%#v", summary)
	}
}

func TestNewServiceValidatesScheduleAndUsesDefaults(t *testing.T) {
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{2}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	collector := fakeCollector{}
	store := &fakeStore{}
	for index, input := range []struct {
		store      Store
		collector  Collector
		timezone   string
		reportHour int
	}{{nil, collector, "Europe/Moscow", 9}, {store, nil, "Europe/Moscow", 9}, {store, collector, "Missing/Timezone", 9}, {store, collector, "Europe/Moscow", -1}, {store, collector, "Europe/Moscow", 24}} {
		if _, createErr := NewService(input.store, input.collector, cipher, nil, input.timezone, input.reportHour, "test"); createErr == nil {
			t.Fatalf("case %d expected error", index)
		}
	}
	if _, createErr := NewService(store, collector, nil, nil, "Europe/Moscow", 9, "test"); createErr == nil {
		t.Fatal("nil notification cipher must be rejected")
	}
	service, err := NewService(store, collector, cipher, nil, "Europe/Moscow", 9, "  release-1  ")
	if err != nil || service.now == nil || service.release != "release-1" {
		t.Fatalf("service=%#v error=%v", service, err)
	}
}

func TestTickNormalizesSampleAndPropagatesStoreFailures(t *testing.T) {
	now := time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	service := monitoringServiceForTest(t, store, fakeCollector{sample: Sample{}}, now)
	if err := service.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.samples[0].Status != "complete" || store.samples[0].Metrics == nil || store.samples[0].Release != "test" {
		t.Fatalf("normalized sample=%#v", store.samples[0])
	}

	saveErr := errors.New("save failed")
	store = &fakeStore{saveErr: saveErr}
	service = monitoringServiceForTest(t, store, fakeCollector{}, now)
	if err := service.Tick(context.Background()); !errors.Is(err, saveErr) {
		t.Fatalf("save error=%v", err)
	}
	aggregateErr := errors.New("aggregate failed")
	store = &fakeStore{aggregateErr: aggregateErr}
	service = monitoringServiceForTest(t, store, fakeCollector{}, now)
	if err := service.Tick(context.Background()); !errors.Is(err, aggregateErr) {
		t.Fatalf("aggregate error=%v", err)
	}
	enqueueErr := errors.New("enqueue failed")
	store = &fakeStore{enqueueErr: enqueueErr}
	service = monitoringServiceForTest(t, store, fakeCollector{}, now)
	if err := service.Tick(context.Background()); !errors.Is(err, enqueueErr) {
		t.Fatalf("enqueue error=%v", err)
	}
}

func TestEvaluateAlertsCoversThresholdStatesAndPropagatesFailure(t *testing.T) {
	now := time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	service := monitoringServiceForTest(t, store, fakeCollector{}, now)
	sample := Sample{Slot: now, Status: "complete", Metrics: map[string]float64{
		"cpu_busy_percent": 96, "ram_working_set_percent": 82, "disk_used_percent": 70,
		"io_wait_percent": 5, "postgres_connections_percent": 90, "containers_unhealthy": 0,
		"container_unplanned_restarts": 1, "failed_login_spike": 0,
		"backup_age_hours": 27, "backup_verified": 1,
	}}
	if err := service.evaluateAlerts(context.Background(), sample); err != nil {
		t.Fatal(err)
	}
	if len(store.alerts) != 9 {
		t.Fatalf("alerts=%#v", store.alerts)
	}
	if !store.alerts[0].ImmediateActivation || store.alerts[len(store.alerts)-1].Type != "metrics.collector" {
		t.Fatalf("alerts=%#v", store.alerts)
	}

	store = &fakeStore{alertErr: errors.New("alert failed")}
	service = monitoringServiceForTest(t, store, fakeCollector{}, now)
	if err := service.evaluateAlerts(context.Background(), Sample{Slot: now, Status: "failed", Metrics: map[string]float64{}}); err == nil {
		t.Fatal("expected alert persistence error")
	}
}

func TestDailyMessageMarksIncompleteDataWithoutPII(t *testing.T) {
	service := monitoringServiceForTest(t, &fakeStore{}, fakeCollector{}, time.Now())
	period := Period{Start: time.Date(2026, 10, 1, 21, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 2, 21, 0, 0, 0, time.UTC)}
	incomplete := service.dailyMessage(DailyAggregate{Period: period, ExpectedSamples: 96, ReceivedSamples: 80, FailedSubsystems: []string{"postgres"}, AccountsTotal: 10, NewRegistrations: 2, SuccessfulLogins: 7, UniqueLoggedIn: 4, FailedLogins: 3, ActiveSessions: 2})
	if !strings.Contains(incomplete, "ДАННЫЕ НЕПОЛНЫЕ") || strings.Contains(incomplete, "postgres") || strings.Contains(incomplete, "@") {
		t.Fatalf("message=%q", incomplete)
	}
	complete := service.dailyMessage(DailyAggregate{Period: period, ExpectedSamples: 96, ReceivedSamples: 96, Metrics: map[string]Summary{"cpu_busy_percent": {P95: 42.5}, "disk_used_percent": {Maximum: 61}}})
	if strings.Contains(complete, "ДАННЫЕ НЕПОЛНЫЕ") || !strings.Contains(complete, "CPU p95 42.5") || !strings.Contains(complete, "диск max 61.0") || !strings.Contains(complete, "RAM p95 n/a") {
		t.Fatalf("complete message=%q", complete)
	}
}

func TestRuntimeCollectorAndEmptySummary(t *testing.T) {
	collector := NewRuntimeCollector("")
	now := time.Now().UTC().Truncate(15 * time.Minute)
	sample, err := collector.Collect(context.Background(), now)
	if err != nil || sample.Status == "" || sample.Metrics["go_goroutines"] < 1 || sample.Metrics["go_heap_bytes"] < 1 {
		t.Fatalf("sample=%#v error=%v", sample, err)
	}
	if delay := UntilNextQuarterHour(time.Date(2026, 10, 9, 12, 7, 30, 0, time.UTC)); delay != 7*time.Minute+30*time.Second {
		t.Fatalf("delay=%v", delay)
	}
	directory := t.TempDir()
	if err := os.WriteFile(directory+"/latest-offsite-success", []byte(now.Add(-2*time.Hour).Format("20060102T150405Z")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if age, ok := verifiedBackupAge(directory, now); !ok || age != 2 {
		t.Fatalf("age=%v ok=%v", age, ok)
	}
	backupCollector := NewRuntimeCollector("  " + directory + "  ")
	first, err := backupCollector.Collect(context.Background(), now)
	if err != nil || first.Metrics["backup_verified"] != 1 || first.Metrics["backup_age_hours"] != 2 {
		t.Fatalf("first backup sample=%#v error=%v", first, err)
	}
	second, err := backupCollector.Collect(context.Background(), now.Add(15*time.Minute))
	if err != nil || second.Metrics["backup_verified"] != 1 {
		t.Fatalf("second backup sample=%#v error=%v", second, err)
	}
	if err = os.WriteFile(directory+"/latest-offsite-success", []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := verifiedBackupAge(directory, now); ok {
		t.Fatal("invalid backup marker must be rejected")
	}
	invalid, err := backupCollector.Collect(context.Background(), now.Add(30*time.Minute))
	if err != nil || invalid.Metrics["backup_verified"] != 0 || invalid.Status != "partial" {
		t.Fatalf("invalid backup sample=%#v error=%v", invalid, err)
	}
	if err = os.WriteFile(directory+"/latest-offsite-success", []byte(now.Add(time.Hour).Format("20060102T150405Z")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := verifiedBackupAge(directory, now); ok {
		t.Fatal("future backup marker must be rejected")
	}
	if got := Summarize(nil); got != (Summary{}) {
		t.Fatalf("summary=%#v", got)
	}
}

func TestEvaluateAlertsRecordsHealthyBackupRecovery(t *testing.T) {
	now := time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	service := monitoringServiceForTest(t, store, fakeCollector{}, now)
	if err := service.evaluateAlerts(context.Background(), Sample{Slot: now, Status: "partial", Metrics: map[string]float64{"backup_age_hours": 1, "backup_verified": 1}}); err != nil {
		t.Fatal(err)
	}
	if len(store.alerts) != 2 || store.alerts[0].Type != "backup.stale" || store.alerts[0].State != "recovered" || store.alerts[1].Type != "metrics.collector" || store.alerts[1].State != "problem" {
		t.Fatalf("alerts=%#v", store.alerts)
	}
}

func TestMonitoringEncryptionFailuresAndSystemProbeFallbacks(t *testing.T) {
	now := time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)
	store := &fakeStore{inserted: true, alertErr: errors.New("alert failed")}
	service := monitoringServiceForTest(t, store, fakeCollector{sample: Sample{Status: "complete", Metrics: map[string]float64{}}}, now)
	if err := service.Tick(context.Background()); err == nil {
		t.Fatal("tick must propagate alert evaluation failure")
	}

	invalidCipherService, err := NewService(&fakeStore{}, fakeCollector{}, failingCipher{}, func() time.Time { return now }, "Europe/Moscow", 9, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err = invalidCipherService.enqueueDueDailyReports(context.Background(), now); err == nil {
		t.Fatal("daily report encryption failure expected")
	}
	for _, sample := range []Sample{
		{Slot: now, Status: "complete", Metrics: map[string]float64{"cpu_busy_percent": 99}},
		{Slot: now, Status: "complete", Metrics: map[string]float64{"backup_age_hours": 30, "backup_verified": 0}},
		{Slot: now, Status: "complete", Metrics: map[string]float64{}},
	} {
		if err = invalidCipherService.evaluateAlerts(context.Background(), sample); err == nil {
			t.Fatalf("incident encryption failure expected for sample=%#v", sample)
		}
	}
	if _, _, ok := filesystemUsage("/definitely/missing/arhdesign/path"); ok {
		t.Fatal("missing filesystem path reported as available")
	}
	if _, ok := verifiedBackupAge(t.TempDir(), now); ok {
		t.Fatal("missing backup marker reported as verified")
	}
	_, _, _ = cgroupMemory()
	_, _, _ = cgroupCPU()
}
