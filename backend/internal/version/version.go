// Package version holds the single source of truth for the WebStats release
// version. It is served by both the dashboard and ingestion APIs and is what
// the dashboard compares against the latest GitHub release to show an
// update notification.
//
// Bump this value (and tag the commit, e.g. v1.2.0) when releasing.
package version

// Version is the current release of WebStats.
const Version = "1.1.0"
