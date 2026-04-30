package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	tests := []struct {
		expr    string
		wantErr bool
		iv      int
	}{
		{"every 5 minutes", false, 300},
		{"every 30 seconds", false, 30},
		{"every 2 hours", false, 7200},
		{"every minute", false, 60},
		{"every hour", false, 3600},
		{"every second", false, 1},
		{"every 1 second", false, 1},
		{"daily at 09:00", false, 0},
		{"daily at 23:59", false, 0},
		{"every monday", false, 0},
		{"every friday at 14:30", false, 0},
		{"every sunday at 00:00", false, 0},
		{"", true, 0},
		{"foo bar", true, 0},
		{"every", true, 0},
		{"daily at 25:00", true, 0},
		{"every foo at 12:00", true, 0},
	}
	for _, tt := range tests {
		s, err := ParseSchedule(tt.expr)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ParseSchedule(%q) expected error", tt.expr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseSchedule(%q) unexpected error: %v", tt.expr, err)
		}
		if s.IntervalSeconds != tt.iv {
			t.Fatalf("ParseSchedule(%q) interval = %d, want %d", tt.expr, s.IntervalSeconds, tt.iv)
		}
	}
}

func TestParseSchedule_CaseInsensitive(t *testing.T) {
	cases := []string{
		"EVERY 5 MINUTES",
		"Every 5 Minutes",
		"every 5 MiNuTeS",
	}
	for _, c := range cases {
		s, err := ParseSchedule(c)
		if err != nil {
			t.Fatalf("ParseSchedule(%q) error: %v", c, err)
		}
		if s.IntervalSeconds != 300 {
			t.Fatalf("ParseSchedule(%q) = %d, want 300", c, s.IntervalSeconds)
		}
	}
}

func TestParseSchedule_DailyAt(t *testing.T) {
	s, err := ParseSchedule("daily at 14:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DailyAt == nil {
		t.Fatal("expected DailyAt to be set")
	}
	if s.DailyAt.Hour() != 14 || s.DailyAt.Minute() != 30 {
		t.Fatalf("DailyAt = %02d:%02d, want 14:30", s.DailyAt.Hour(), s.DailyAt.Minute())
	}
}

func TestParseSchedule_Weekly(t *testing.T) {
	s, err := ParseSchedule("every tuesday at 10:15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.WeeklyDay != 1 {
		t.Fatalf("WeeklyDay = %d, want 1 (tuesday)", s.WeeklyDay)
	}
	if s.WeeklyHour != 10 || s.WeeklyMinute != 15 {
		t.Fatalf("Weekly time = %02d:%02d, want 10:15", s.WeeklyHour, s.WeeklyMinute)
	}
}

func TestNextRunDelay_Interval(t *testing.T) {
	s := &Schedule{IntervalSeconds: 120}
	d := NextRunDelay(s)
	if d != 2*time.Minute {
		t.Fatalf("expected 2m, got %v", d)
	}
}

func TestNextRunDelay_DailyAt(t *testing.T) {
	now := time.Now()
	// Set target to 10 minutes from now (truncate to minute because DailyAt has 0 seconds)
	target := now.Truncate(time.Minute).Add(10 * time.Minute)
	s := &Schedule{DailyAt: &target}
	d := NextRunDelay(s)
	// Should be roughly 10 minutes (allow 2m tolerance for test execution time and clock skew)
	if d < 8*time.Minute || d > 12*time.Minute {
		t.Fatalf("expected ~10m, got %v", d)
	}
}

func TestNextRunDelay_DailyAtTomorrow(t *testing.T) {
	now := time.Now()
	// Set target to 10 minutes ago (should be tomorrow)
	target := now.Truncate(time.Minute).Add(-10 * time.Minute)
	s := &Schedule{DailyAt: &target}
	d := NextRunDelay(s)
	// Should be roughly 23h50m
	if d < 23*time.Hour+30*time.Minute {
		t.Fatalf("expected ~23h50m, got %v", d)
	}
}

func TestNextRunDelay_Weekly(t *testing.T) {
	now := time.Now()
	// Same day, 1 minute from now
	s := &Schedule{WeeklyDay: int(now.Weekday()), WeeklyHour: now.Hour(), WeeklyMinute: now.Minute() + 1}
	d := NextRunDelay(s)
	if d < 0 || d > 2*time.Minute {
		t.Fatalf("expected ~1m, got %v", d)
	}
}

func TestCollectScheduledHunts(t *testing.T) {
	dir := t.TempDir()
	content := `@context: test
@schedule: every 1 minute

STEP 1:
    NAVIGATE to https://example.com

DONE.
`
	_ = os.WriteFile(filepath.Join(dir, "test.hunt"), []byte(content), 0644)
	_ = os.WriteFile(filepath.Join(dir, "no_schedule.hunt"), []byte("STEP 1:\n    WAIT 1\nDONE.\n"), 0644)

	hunts, err := CollectScheduledHunts(dir)
	if err != nil {
		t.Fatalf("CollectScheduledHunts error: %v", err)
	}
	if len(hunts) != 1 {
		t.Fatalf("expected 1 scheduled hunt, got %d", len(hunts))
	}
	if hunts[0].Schedule.IntervalSeconds != 60 {
		t.Fatalf("expected 60s interval, got %d", hunts[0].Schedule.IntervalSeconds)
	}
}

func TestCollectScheduledHunts_Subdirectories(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	_ = os.MkdirAll(sub, 0755)

	_ = os.WriteFile(filepath.Join(dir, "root.hunt"), []byte("@schedule: every 5 minutes\nSTEP 1:\n    WAIT 1\nDONE.\n"), 0644)
	_ = os.WriteFile(filepath.Join(sub, "nested.hunt"), []byte("@schedule: every 10 minutes\nSTEP 1:\n    WAIT 1\nDONE.\n"), 0644)

	hunts, err := CollectScheduledHunts(dir)
	if err != nil {
		t.Fatalf("CollectScheduledHunts error: %v", err)
	}
	if len(hunts) != 2 {
		t.Fatalf("expected 2 scheduled hunts, got %d", len(hunts))
	}
	// Verify paths include subdirectories
	foundNested := false
	for _, h := range hunts {
		if strings.Contains(h.Path, "sub/nested.hunt") {
			foundNested = true
		}
	}
	if !foundNested {
		t.Fatal("expected to find nested hunt in subdirectory")
	}
}

func TestCollectScheduledHunts_InvalidSchedule(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "bad.hunt"), []byte("@schedule: invalid schedule\nSTEP 1:\n    WAIT 1\nDONE.\n"), 0644)

	hunts, err := CollectScheduledHunts(dir)
	if err != nil {
		t.Fatalf("CollectScheduledHunts error: %v", err)
	}
	if len(hunts) != 0 {
		t.Fatalf("expected 0 scheduled hunts (invalid schedule skipped), got %d", len(hunts))
	}
}

func TestCollectScheduledHunts_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	hunts, err := CollectScheduledHunts(dir)
	if err != nil {
		t.Fatalf("CollectScheduledHunts error: %v", err)
	}
	if len(hunts) != 0 {
		t.Fatalf("expected 0 scheduled hunts in empty dir, got %d", len(hunts))
	}
}

func TestCollectScheduledHunts_ParseError(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "broken.hunt"), []byte("this is not valid hunt syntax!!!\n"), 0644)

	hunts, err := CollectScheduledHunts(dir)
	if err != nil {
		t.Fatalf("CollectScheduledHunts error: %v", err)
	}
	if len(hunts) != 0 {
		t.Fatalf("expected 0 scheduled hunts (parse error skipped), got %d", len(hunts))
	}
}
