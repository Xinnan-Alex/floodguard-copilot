export interface FeedItem {
  id: string;
  type: 'call' | 'whatsapp' | 'claim';
  timestamp: string;
  preview: string;
  fullData: string;
  imageObjectUrl?: string;
  imageBase64?: string;
  imageMimeType?: string;
  mediaUrl?: string;
  from?: string;
}

export interface TriageResult {
  location?: string;
  urgency?: number;
  needs?: string;
  status?: string;
  raw_extraction?: string;
  amount?: number;
  reasoning?: string;
  suggested_action?: string;
}

export interface DispatchResult {
  confirmation_id?: string;
  action_type?: string;
  summary?: string;
  routed_to?: string;
  eta_minutes?: number;
  status?: string;
}
