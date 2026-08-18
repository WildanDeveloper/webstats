export type Site = {
  id: string;
  user_id: string;
  name: string;
  domain: string;
  site_key: string;
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