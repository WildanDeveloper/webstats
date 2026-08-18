export type Site = {
  id: string;
  user_id: string;
  name: string;
  domain: string;
  site_key: string;
  color: string;
  created_at: string;
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
};

export type TimePoint = {
  date: string;
  pageviews: number;
  visitors: number;
};

export type Row = {
  key: string;
  value: number;
};

export type EventRow = {
  name: string;
  count: number;
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