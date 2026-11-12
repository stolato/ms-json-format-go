package tasks

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestCleanupScheduleRunsDailyAt0100(t *testing.T) {
	sched, err := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(cleanupSchedule)
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 11, 11, 17, 30, 0, 0, time.UTC)
	next := sched.Next(from)
	want := time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("próxima execução = %v, esperado %v", next, want)
	}
	if after := sched.Next(next); !after.Equal(want.Add(24 * time.Hour)) {
		t.Fatalf("execução seguinte = %v, esperado %v", after, want.Add(24*time.Hour))
	}
}
