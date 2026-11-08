package devices

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageReportsMissingMountPoints(t *testing.T) {
	assert.Equal(t, "ENOENT", statfs("/definitely/not/here").Code)
}

func TestUsageIncludesEveryMountPoint(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	service := NewService(nil, testUID)
	service.objects = table()
	delete(service.objects, "/block/home")
	service.objects["/block/root"][FsIface]["MountPoints"] = mountPoints(first, second)

	usage := service.Usage(nil, false)
	require.Len(t, usage, 2)
	assert.ElementsMatch(t, []string{first, second}, []string{usage[0].MountPoint, usage[1].MountPoint})
	for _, entry := range usage {
		assert.Empty(t, entry.Code)
		assert.Greater(t, entry.Total, uint64(0))
	}
}

func TestUsageCacheHoldsForThirtySeconds(t *testing.T) {
	cache := newUsageCache()
	now := time.Now()
	cache.now = func() time.Time { return now }

	dir := t.TempDir()
	first := cache.get(dir, false)
	cache.entries[dir] = usageEntry{usage: Usage{MountPoint: dir, Total: 42}, at: now}

	assert.Equal(t, uint64(42), cache.get(dir, false).Total, "a fresh entry is reused")
	assert.Equal(t, first.Total, cache.get(dir, true).Total, "force re-reads")

	cache.entries[dir] = usageEntry{usage: Usage{MountPoint: dir, Total: 42}, at: now}
	now = now.Add(usageTTL + time.Second)
	assert.Equal(t, first.Total, cache.get(dir, false).Total, "a stale entry is re-read")
}
