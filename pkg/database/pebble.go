package database

import (
	"time"

	pebbleDB "github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"

	"github.com/iotaledger/hive.go/core/kvstore/pebble"
)

// NewPebbleDB creates a new pebble DB instance.
func NewPebbleDB(directory string, reportCompactionRunning func(running bool), enableFilter bool) (*pebbleDB.DB, error) {
	cache := pebbleDB.NewCache(128 << 20) // 128 MB
	defer cache.Unref()

	opts := &pebbleDB.Options{}
	opts.EnsureDefaults()

	// Per-level options. Options for at least one level must be specified. The
	// options for the last level are used for all subsequent levels.
	opts.Levels = make([]pebbleDB.LevelOptions, 1)
	for i := 0; i < len(opts.Levels); i++ {
		l := &opts.Levels[i]
		l.EnsureDefaults()

		// Compression defines the per-block compression to use.
		//
		// The default value (DefaultCompression) uses snappy compression.
		l.Compression = pebbleDB.NoCompression

		// FilterPolicy defines a filter algorithm (such as a Bloom filter) that can
		// reduce disk reads for Get calls.
		//
		// One such implementation is bloom.FilterPolicy(10) from the pebble/bloom
		// package.
		//
		// The default value means to use no filter.
		if enableFilter {
			l.FilterPolicy = bloom.FilterPolicy(10)
		}

		// FilterType defines whether an existing filter policy is applied at a
		// block-level or table-level. Block-level filters use less memory to create,
		// but are slower to access as a check for the key in the index must first be
		// performed to locate the filter block. A table-level filter will require
		// memory proportional to the number of keys in an sstable to create, but
		// avoids the index lookup when determining if a key is present. Table-level
		// filters should be preferred except under constrained memory situations.
		l.FilterType = pebbleDB.TableFilter

		// The target file size for the level.
		// The default value is twice as big as the level before.
		if i > 0 {
			l.TargetFileSize = opts.Levels[i-1].TargetFileSize * 2
		}
	}

	// Sync sstables periodically in order to smooth out writes to disk. This
	// option does not provide any persistency guarantee, but is used to avoid
	// latency spikes if the OS automatically decides to write out a large chunk
	// of dirty filesystem buffers. This option only controls SSTable syncs; WAL
	// syncs are controlled by WALBytesPerSync.
	//
	// The default value is 512KB.
	opts.BytesPerSync = 512 << 10 // 512 KB

	// Cache is used to cache uncompressed blocks from sstables.
	//
	// The default cache size is 8 MB.
	opts.Cache = cache // 128 MB

	// Disable the write-ahead log (WAL). Disabling the write-ahead log prohibits
	// crash recovery, but can improve performance if crash recovery is not
	// needed (e.g. when only temporary state is being stored in the database).
	//
	// The default value is false.
	opts.DisableWAL = true

	// EventListener provides hooks to listening to significant DB events such as
	// flushes, compactions, and table deletion.
	opts.EventListener.CompactionBegin = func(info pebbleDB.CompactionInfo) {
		if reportCompactionRunning != nil {
			reportCompactionRunning(true)
		}
	}
	opts.EventListener.CompactionEnd = func(info pebbleDB.CompactionInfo) {
		if reportCompactionRunning != nil {
			reportCompactionRunning(false)
		}
	}

	// The threshold of L0 read-amplification at which compaction concurrency
	// is enabled (if CompactionDebtConcurrency was not already exceeded).
	// Every multiple of this value enables another concurrent
	// compaction up to MaxConcurrentCompactions.
	//
	// The default value is 10.
	opts.Experimental.L0CompactionConcurrency = 100

	// CompactionDebtConcurrency controls the threshold of compaction debt
	// at which additional compaction concurrency slots are added. For every
	// multiple of this value in compaction debt bytes, an additional
	// concurrent compaction is added. This works "on top" of
	// L0CompactionConcurrency, so the higher of the count of compaction
	// concurrency slots as determined by the two options is chosen.
	//
	// The default value is 1 GB.
	opts.Experimental.CompactionDebtConcurrency = 10 << 30 // 10 GB

	// MinDeletionRate is the minimum number of bytes per second that would
	// be deleted. Deletion pacing is used to slow down deletions when
	// compactions finish up or readers close, and newly-obsolete files need
	// cleaning up. Deleting lots of files at once can cause disk latency to
	// go up on some SSDs, which this functionality guards against. This is a
	// minimum as the maximum is theoretically unlimited; pacing is disabled
	// when there are too many obsolete files relative to live bytes, or
	// there isn't enough disk space available. Setting this to 0 disables
	// deletion pacing, which is also the default.
	//
	// The default value is 0.
	opts.Experimental.MinDeletionRate = 0

	// ReadCompactionRate controls the frequency of read triggered
	// compactions by adjusting `AllowedSeeks` in manifest.FileMetadata:
	//
	// AllowedSeeks = FileSize / ReadCompactionRate
	//
	// From LevelDB:
	// ```
	// We arrange to automatically compact this file after
	// a certain number of seeks. Let's assume:
	//   (1) One seek costs 10ms
	//   (2) Writing or reading 1MB costs 10ms (100MB/s)
	//   (3) A compaction of 1MB does 25MB of IO:
	//         1MB read from this level
	//         10-12MB read from next level (boundaries may be misaligned)
	//         10-12MB written to next level
	// This implies that 25 seeks cost the same as the compaction
	// of 1MB of data.  I.e., one seek costs approximately the
	// same as the compaction of 40KB of data.  We are a little
	// conservative and allow approximately one seek for every 16KB
	// of data before triggering a compaction.
	// ```
	//
	// The default value is 16000.
	opts.Experimental.ReadCompactionRate = 1

	// ReadSamplingMultiplier is a multiplier for the readSamplingPeriod in
	// iterator.maybeSampleRead() to control the frequency of read sampling
	// to trigger a read triggered compaction. A value of -1 prevents sampling
	// and disables read triggered compactions.
	//
	// The default value is 1.
	opts.Experimental.ReadSamplingMultiplier = 0

	// DeleteRangeFlushDelay configures how long the database should wait
	// before forcing a flush of a memtable that contains a range
	// deletion. Disk space cannot be reclaimed until the range deletion
	// is flushed. No automatic flush occurs if zero.
	//
	// The default value is 0.
	opts.FlushDelayDeleteRange = 10 * time.Second

	// FlushSplitBytes denotes the target number of bytes per sublevel in
	// each flush split interval (i.e. range between two flush split keys)
	// in L0 sstables. When set to zero, only a single sstable is generated
	// by each flush. When set to a non-zero value, flushes are split at
	// points to meet L0's TargetFileSize, any grandparent-related overlap
	// options, and at boundary keys of L0 flush split intervals (which are
	// targeted to contain around FlushSplitBytes bytes in each sublevel
	// between pairs of boundary keys). Splitting sstables during flush
	// allows increased compaction flexibility and concurrency when those
	// tables are compacted to lower levels.
	//
	// The default value is 2 * opts.Levels[0].TargetFileSize.
	opts.FlushSplitBytes = 2 * opts.Levels[0].TargetFileSize

	// The amount of L0 read-amplification necessary to trigger an L0 compaction.
	//
	// The default value is 4.
	opts.L0CompactionThreshold = 20

	// Hard limit on L0 read-amplification. Writes are stopped when this
	// threshold is reached. If Experimental.L0SublevelCompactions is enabled
	// this threshold is measured against the number of L0 sublevels. Otherwise
	// it is measured against the number of files in L0.
	//
	// The default value is 12.
	opts.L0StopWritesThreshold = 200

	// The maximum number of bytes for LBase. The base level is the level which
	// L0 is compacted into. The base level is determined dynamically based on
	// the existing data in the LSM. The maximum number of bytes for other levels
	// is computed dynamically based on the base level's maximum size. When the
	// maximum number of bytes for a level is exceeded, compaction is requested.
	//
	// The default value is 64 MB.
	opts.LBaseMaxBytes = 64 << 20 // 64 MB

	// MaxManifestFileSize is the maximum size the MANIFEST file is allowed to
	// become. When the MANIFEST exceeds this size it is rolled over and a new
	// MANIFEST is created.
	//
	// The default value is 128 MB.
	opts.MaxManifestFileSize = 128 << 20 // 128 MB

	// MaxOpenFiles is a soft limit on the number of open files that can be
	// used by the DB.
	//
	// The default value is 1000.
	opts.MaxOpenFiles = 16384

	// The size of a MemTable in steady state. The actual MemTable size starts at
	// min(256KB, MemTableSize) and doubles for each subsequent MemTable up to
	// MemTableSize. This reduces the memory pressure caused by MemTables for
	// short lived (test) DB instances. Note that more than one MemTable can be
	// in existence since flushing a MemTable involves creating a new one and
	// writing the contents of the old one in the
	// background. MemTableStopWritesThreshold places a hard limit on the size of
	// the queued MemTables.
	//
	// The default value is 4 MB.
	opts.MemTableSize = 8 << 20

	// Hard limit on the size of queued of MemTables. Writes are stopped when the
	// sum of the queued memtable sizes exceeds
	// MemTableStopWritesThreshold*MemTableSize. This value should be at least 2
	// or writes will stop whenever a MemTable is being flushed.
	//
	// The default value is 2.
	opts.MemTableStopWritesThreshold = 2

	// MaxConcurrentCompactions specifies the maximum number of concurrent
	// compactions. The default is 1. Concurrent compactions are only performed
	// when L0 read-amplification passes the L0CompactionConcurrency threshold.
	//
	// The default value is 1.
	opts.MaxConcurrentCompactions = func() int { return 1 }

	return pebble.CreateDB(directory, opts)
}
