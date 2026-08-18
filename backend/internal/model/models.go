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
