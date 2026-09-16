package model

import "time"

type User struct {
	ID       string    `json:"id"`
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	Role     string    `json:"role"`
	Password string    `json:"-"`
	Created  time.Time `json:"created_at"`
}

type Site struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Domain    string    `json:"domain"`
	SiteKey   string    `json:"site_key"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status,omitempty"`
	LatencyMs int64     `json:"latency_ms,omitempty"`
	CheckedAt time.Time `json:"checked_at,omitempty"`
}

type Overview struct {
	Pageviews     int64   `json:"pageviews"`
	Visitors      int64   `json:"visitors"`
	Sessions      int64   `json:"sessions"`
	Bounces       int64   `json:"bounces"`
	BounceRate    float64 `json:"bounce_rate"`
	AvgPerDay     float64 `json:"avg_per_day"`
	PrevPageviews int64   `json:"prev_pageviews"`
	PrevVisitors  int64   `json:"prev_visitors"`
	PrevSessions  int64   `json:"prev_sessions"`
	PrevBounces   int64   `json:"prev_bounces"`
}

type TimePoint struct {
	Date          string `json:"date"`
	Pageviews     int64  `json:"pageviews"`
	Visitors      int64  `json:"visitors"`
	PrevPageviews int64  `json:"prev_pageviews"`
	PrevVisitors  int64  `json:"prev_visitors"`
}

type Row struct {
	Key   string `json:"key"`
	Value int64  `json:"value"`
}

type WorldPoint struct {
	Country string  `json:"country"`
	Count   int64   `json:"count"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

type EventRow struct {
	Name   string `json:"name"`
	Count  int64  `json:"count"`
	LastAt string `json:"last_at"`
}

type EventDetail struct {
	Name     string  `json:"name"`
	Count    int64   `json:"count"`
	Visitors int64   `json:"visitors"`
	AvgValue float64 `json:"avg_value"`
	MaxValue float64 `json:"max_value"`
	MinValue float64 `json:"min_value"`
}

type EventOccurrence struct {
	Name      string         `json:"name"`
	SessionID string         `json:"session_id"`
	URL       string         `json:"url"`
	Props     map[string]any `json:"props"`
	CreatedAt time.Time      `json:"created_at"`
}

type SiteSettings struct {
	SiteID        string `json:"site_id"`
	IPHashing     bool   `json:"ip_hashing"`
	RetentionDays int    `json:"retention_days"`
	PublicToken   string `json:"public_token"`
	PublicEnabled bool   `json:"public_enabled"`
}

type Member struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	IsOwner   bool      `json:"is_owner"`
	CreatedAt time.Time `json:"created_at"`
}

type Invite struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"site_id"`
	SiteName  string    `json:"site_name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Token     string    `json:"token"`
	InviteURL string    `json:"invite_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Realtime struct {
	Visitors  int64 `json:"visitors"`
	Pageviews int64 `json:"pageviews"`
	Pages     []Row `json:"pages"`
	Countries []Row `json:"countries"`
}

type Check struct {
	ID        int64     `json:"id"`
	SiteID    string    `json:"site_id"`
	Status    string    `json:"status"`
	LatencyMs int64     `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at"`
}

type FunnelStep struct {
	ID       string `json:"id"`
	SiteID   string `json:"site_id"`
	Position int    `json:"position"`
	Label    string `json:"label"`
}

type Funnel struct {
	ID     string       `json:"id"`
	SiteID string       `json:"site_id"`
	Steps  []FunnelStep `json:"steps"`
}

type Monitor struct {
	ID              string       `json:"id"`
	SiteID          string       `json:"site_id"`
	URL             string       `json:"url"`
	IntervalSeconds int          `json:"interval_seconds"`
	ExpectedStatus  int          `json:"expected_status"`
	Keyword         string       `json:"keyword"`
	KeywordMode     string       `json:"keyword_mode"`
	Enabled         bool         `json:"enabled"`
	LastStatus      *int         `json:"last_status"`
	LastOK          *bool        `json:"last_ok"`
	LastCheckAt     *time.Time   `json:"last_check_at"`
	UptimePct       float64      `json:"uptime_pct"`
	Days            []MonitorDay `json:"days,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
}

type MonitorDay struct {
	Date  string `json:"date"`
	Up    int64  `json:"up"`
	Total int64  `json:"total"`
}

type MonitorCheck struct {
	StatusCode int       `json:"status_code"`
	OK         bool      `json:"ok"`
	LatencyMs  int       `json:"latency_ms"`
	CheckedAt  time.Time `json:"checked_at"`
}

type Heartbeat struct {
	ID            string     `json:"id"`
	SiteID        string     `json:"site_id"`
	Name          string     `json:"name"`
	PeriodSeconds int        `json:"period_seconds"`
	GraceSeconds  int        `json:"grace_seconds"`
	PingKey       string     `json:"ping_key"`
	PingURL       string     `json:"ping_url,omitempty"`
	LastPingAt    *time.Time `json:"last_ping_at"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
}

type MaintenanceWindow struct {
	ID          string    `json:"id"`
	SiteID      string    `json:"site_id"`
	Days        []string  `json:"days"`
	StartMinute int       `json:"start_minute"`
	EndMinute   int       `json:"end_minute"`
	CreatedAt   time.Time `json:"created_at"`
}

type Incident struct {
	ID         int64      `json:"id"`
	SiteID     string     `json:"site_id"`
	MonitorID  *string    `json:"monitor_id,omitempty"`
	Kind       string     `json:"kind"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	Reason     string     `json:"reason"`
}

type ApiKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type InsightHighlight struct {
	Kind     string  `json:"kind"`
	Title    string  `json:"title"`
	Text     string  `json:"text"`
	DeltaPct float64 `json:"delta_pct"`
}

type Insights struct {
	Summary    string             `json:"summary"`
	Highlights []InsightHighlight `json:"highlights"`
}

type PublicSiteInfo struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Color  string `json:"color"`
}

type VitalStat struct {
	Metric  string  `json:"metric"`
	P75     float64 `json:"p75"`
	Samples int64   `json:"samples"`
}

type VitalPathRow struct {
	Path string  `json:"path"`
	LCP  float64 `json:"lcp"`
	CLS  float64 `json:"cls"`
	INP  float64 `json:"inp"`
	N    int64   `json:"n"`
}

type VitalTrendPoint struct {
	Date string  `json:"date"`
	LCP  float64 `json:"lcp"`
	CLS  float64 `json:"cls"`
	INP  float64 `json:"inp"`
}

type Vitals struct {
	Summary []VitalStat       `json:"summary"`
	Paths   []VitalPathRow    `json:"paths"`
	Trend   []VitalTrendPoint `json:"trend"`
}

type SiteRank struct {
	SiteID        string `json:"site_id"`
	Name          string `json:"name"`
	Color         string `json:"color"`
	Pageviews     int64  `json:"pageviews"`
	Visitors      int64  `json:"visitors"`
	PrevPageviews int64  `json:"prev_pageviews"`
}
