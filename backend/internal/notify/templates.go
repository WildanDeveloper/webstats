package notify

import "fmt"

var EventLabels = map[string]string{
	"site_down":     "Site is down",
	"site_up":       "Site is back online",
	"traffic_spike": "Traffic spike detected",
}

type AlertPayload struct {
	Event     string `json:"event"`
	SiteID    string `json:"site_id"`
	SiteName  string `json:"site_name"`
	Domain    string `json:"domain"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Count     int64  `json:"count,omitempty"`
	Avg       int64  `json:"avg,omitempty"`
	Threshold int64  `json:"threshold,omitempty"`
	Time      string `json:"time"`
}

func Email(p AlertPayload) string {
	title := EventLabels[p.Event]
	if title == "" {
		title = p.Event
	}
	rows := ""
	add := func(k, v string) {
		rows += fmt.Sprintf(`<tr><td style="padding:8px 16px;border-bottom:1px solid #e5e7eb;color:#6b7280;font-size:14px">%s</td><td style="padding:8px 16px;border-bottom:1px solid #e5e7eb;color:#111827;font-size:14px;font-weight:600">%s</td></tr>`, k, v)
	}
	add("Site", p.SiteName)
	add("Domain", p.Domain)
	add("Status", p.Status)
	add("Time", p.Time)
	if p.LatencyMs > 0 {
		add("Latency", fmt.Sprintf("%d ms", p.LatencyMs))
	}
	if p.Count > 0 {
		add("Pageviews (last hour)", fmt.Sprint(p.Count))
		add("Average (per hour)", fmt.Sprint(p.Avg))
		add("Threshold", fmt.Sprintf("%dx", p.Threshold))
	}
	return `<!DOCTYPE html><html><body style="margin:0;background:#f3f4f6;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif">
<div style="max-width:520px;margin:24px auto;background:#ffffff;border:1px solid #e5e7eb;border-radius:12px;overflow:hidden">
<div style="background:#111827;color:#ffffff;padding:16px 24px;font-size:16px;font-weight:700">` + title + `</div>
<table style="width:100%;border-collapse:collapse">` + rows + `</table>
<div style="padding:16px 24px;color:#9ca3af;font-size:12px">Sent by WebStats</div>
</div></body></html>`
}

func EmailSubject(p AlertPayload) string {
	label := EventLabels[p.Event]
	if label == "" {
		label = p.Event
	}
	return fmt.Sprintf("[WebStats] %s: %s", label, p.Domain)
}
