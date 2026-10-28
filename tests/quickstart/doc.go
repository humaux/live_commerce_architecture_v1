// Package quickstarttests owns OC-05 real-PostgreSQL quick-start marker and HTTP gates.
// Depends on: internal/httpapi, internal/platform, migrations; a disposable CI PostgreSQL database.
// Used by: GitHub Gates shard:unit. Invariants: synthetic fixtures only; missing PG is NOT_RUN.
// It never treats unavailable PostgreSQL as a passing gate or uses real buyer data.
package quickstarttests
