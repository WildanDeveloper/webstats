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
}

type Overview struct {
	Pageviews  int64   `json:"pageviews"`
	Visitors   int64   `json:"visitors"`
	Sessions   int64   `json:"sessions"`
	Bounces    int64   `json:"bounces"`
	BounceRate float64 `json:"bounce_rate"`
	AvgPerDay  float64 `json:"avg_per_day"`
}

type TimePoint struct {
	Date      string `json:"date"`
	Pageviews int64  `json:"pageviews"`
	Visitors  int64  `json:"visitors"`
}

type Row struct {
	Key   string `json:"key"`
	Value int64  `json:"value"`
}

type EventRow struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}
