package clock_test

import (
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/clock"
	"github.com/stretchr/testify/assert"
)

func TestClockSourceSwappable(t *testing.T) {
	defer clock.Reset()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock.SetSource(clock.Fixed(t0))

	// Now returns the pinned time, and standard time.Time methods still work
	// on the returned value.
	got := clock.Now()
	assert.Equal(t, t0, got)
	assert.True(t, got.After(t0.Add(-time.Hour)))
	assert.Equal(t, t0.Add(time.Minute), got.Add(time.Minute))

	// Reset restores the real, moving clock.
	clock.Reset()
	assert.WithinDuration(t, time.Now(), clock.Now(), time.Second)
}

func TestClockInDefaultLocation(t *testing.T) {
	orig := clock.DefaultLocation
	defer func() { clock.DefaultLocation = orig }()

	wib := time.FixedZone("WIB", 7*3600)
	clock.DefaultLocation = wib

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	local := clock.InDefaultLocation(t0)
	assert.Equal(t, wib, local.Location())
	assert.Equal(t, 7, local.Hour())
}
