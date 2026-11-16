-- Host samples and deployment diagnoses are bounded, local evidence. They are
-- never shipped off the instance and never include application environment
-- values.

CREATE TABLE metric_samples (
 sampled_at INTEGER PRIMARY KEY,
 cpu_percent REAL NOT NULL DEFAULT -1,
 memory_used INTEGER NOT NULL DEFAULT 0,
 memory_total INTEGER NOT NULL DEFAULT 0,
 disk_used INTEGER NOT NULL DEFAULT 0,
 disk_total INTEGER NOT NULL DEFAULT 0,
 load1 REAL NOT NULL DEFAULT 0,
 containers_running INTEGER NOT NULL DEFAULT 0,
 containers_total INTEGER NOT NULL DEFAULT 0
);
