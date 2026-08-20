export type Site = {
  id: string;
  user_id: string;
  name: string;
  domain: string;
  site_key: string;
  color: string;
  created_at: string;
  status?: string;
  latency_ms?: number;
  checked_at?: string;
};

export type User = {
  id: string;
  email: string;
  name: string;
  role: string;
  created_at: string;
};

export type Overview = {
  pageviews: number;
  visitors: number;
  sessions: number;
  bounces: number;
  bounce_rate: number;
  avg_per_day: number;
  prev_pageviews: number;
  prev_visitors: number;
};

export type TimePoint = {
  date: string;
  pageviews: number;
  visitors: number;
  prev_pageviews: number;
  prev_visitors: number;
};

export type Row = {
  key: string;
  value: number;
};

export type EventRow = {
  name: string;
  count: number;
  last_at: string;
};

export type EventDetail = {
  name: string;
  count: number;
  visitors: number;
  avg_value: number;
  max_value: number;
  min_value: number;
};

export type EventOccurrence = {
  name: string;
  session_id: string;
  url: string;
  props: Record<string, any>;
  created_at: string;
};

export type SiteSettings = {
  site_id: string;
  ip_hashing: boolean;
  retention_days: number;
  public_token: string;
  public_enabled: boolean;
};

export type FunnelConfig = {
  id: string;
  site_id: string;
  position: number;
  label: string;
};

export type Monitor = {
  id: string;
  site_id: string;
  url: string;
  interval_seconds: number;
  expected_status: number;
  enabled: boolean;
  last_status: number | null;
  last_ok: boolean | null;
  last_check_at: string | null;
  uptime_pct: number;
  created_at: string;
};

export type MonitorCheck = {
  status_code: number;
  ok: boolean;
  latency_ms: number;
  checked_at: string;
};

export type ApiKey = {
  id: string;
  name: string;
  prefix: string;
  created_at: string;
  last_used_at: string | null;
};

export type InsightHighlight = {
  kind: string;
  title: string;
  text: string;
  delta_pct: number;
};

export type Insights = {
  summary: string;
  highlights: InsightHighlight[];
};

export type PublicStatus = {
  site: { name: string; domain: string; color: string };
  monitors: Monitor[];
};

export type Member = {
  user_id: string;
  email: string;
  name: string;
  role: string;
  is_owner: boolean;
  created_at: string;
};

export type Invite = {
  id: string;
  site_id: string;
  site_name: string;
  email: string;
  role: string;
  token: string;
  invite_url: string;
  created_at: string;
  expires_at: string;
};

export type SiteSeries = {
  site_id: string;
  name: string;
  color: string;
  points: TimePoint[];
};

export type RootOverview = {
  pageviews: number;
  visitors: number;
  sites: number;
  events: number;
  series: SiteSeries[];
};

export type SslResult = {
  valid: boolean;
  url: string;
  issuer: string;
  subject: string;
  expires_at: string;
  days_left: number;
  error?: string;
};

export type AdminStats = {
  users: number;
  sites: number;
  pageviews: number;
  events: number;
};

export type VisitorDetail = {
  ip: string;
  isp: string;
  country: string;
  browser: string;
  os: string;
  device: string;
  screen: string;
  lang: string;
  session_id: string;
  first_seen: string;
  last_seen: string;
  pageviews: number;
  sessions: number;
  paths: Row[];
  history: Visitor[];
  country_code: string;
};

export type Visitor = {
  ip: string;
  session_id: string;
  country: string;
  browser: string;
  os: string;
  device: string;
  path: string;
  visited_at: string;
};

export type Realtime = {
  visitors: number;
  pageviews: number;
  pages: Row[];
  countries: Row[];
};

export type WorldPoint = {
  country: string;
  count: number;
  lat: number;
  lng: number;
};

export type Check = {
  id: number;
  site_id: string;
  status: string;
  latency_ms: number;
  checked_at: string;
};

export type NotifProvider = {
  id: string;
  name: string;
  kind: string;
  config: Record<string, any>;
  from_email: string;
  created_at: string;
};

export type NotifRule = {
  id: string;
  site_id: string;
  site_name: string;
  domain: string;
  event: string;
  channel: string;
  target: string;
  provider_id: string;
  provider_name: string;
  params: Record<string, any>;
  enabled: boolean;
  last_sent_at: string | null;
};

export type NotifLog = {
  id: number;
  event: string;
  channel: string;
  status: string;
  detail: string;
  created_at: string;
  site_name: string;
  domain: string;
};

export type Campaign = {
  source: string;
  medium: string;
  campaign: string;
  content: string;
  count: number;
  visitors: number;
};

export type Goal = {
  id: string;
  site_id: string;
  name: string;
  path: string;
  match_type: string;
  created_at: string;
};

export type GoalSummary = Goal & {
  pageviews: number;
  conversions: number;
  conversion_pct: number;
};

export type FunnelStep = {
  path: string;
  label: string;
  sessions: number;
};

export type Report = {
  id: string;
  site_id: string;
  site_name: string;
  domain: string;
  provider_id: string;
  provider_name: string;
  recipient: string;
  frequency: string;
  day: string;
  hour: number;
  enabled: boolean;
  last_sent_at: string | null;
};