import type { FeedItem, TriageResult } from '../types';
import { BrainCircuit, MapPin, Gauge, Stethoscope, FileSearch, Loader2, Inbox } from 'lucide-react';

interface TriagePanelProps {
  activeItem: FeedItem | null;
  loading: boolean;
  triageData: TriageResult | null;
}

export default function TriagePanel({ activeItem, loading, triageData }: TriagePanelProps) {
  return (
    <div className="glass-panel" style={{ overflowY: 'auto' }}>
      <div className="panel-title">
        <BrainCircuit size={13} /> Agentic Intelligence
      </div>
      <div className="triage-insight">
        {!activeItem ? (
          <div className="empty-state">
            <Inbox size={36} />
            <p>Select an incoming feed to analyze</p>
          </div>
        ) : loading ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: '1rem' }}>
            <Loader2 size={28} style={{ color: 'var(--accent)', animation: 'spin 1s linear infinite' }} />
            <div style={{ fontSize: '0.875rem', color: 'var(--text-muted)' }}>Gemini Reasoning Engine running…</div>
          </div>
        ) : triageData ? (
          <div className={`insight-card ${triageData.urgency && triageData.urgency > 7 ? 'high' : 'medium'}`}>
            <h3>Situation Triage Summary</h3>

            {activeItem.imageObjectUrl && (
              <div style={{ textAlign: 'center', marginBottom: '1rem' }}>
                <img src={activeItem.imageObjectUrl} alt="Analyzed" style={{ maxHeight: '150px', borderRadius: '8px' }} />
              </div>
            )}

            <div className="data-row">
              <span className="data-label"><MapPin size={10} style={{ display: 'inline', marginRight: '0.3rem', verticalAlign: 'middle' }} />Location</span>
              <span className="data-value">{triageData.location || 'N/A'}</span>
            </div>
            <div className="data-row">
              <span className="data-label"><Gauge size={10} style={{ display: 'inline', marginRight: '0.3rem', verticalAlign: 'middle' }} />Urgency</span>
              <span className="data-value" style={{ color: triageData.urgency && triageData.urgency > 7 ? 'var(--danger)' : 'var(--warning)' }}>{triageData.urgency || 'N/A'} / 10</span>
            </div>
            {triageData.urgency && (
              <div className="urgency-bar-wrap" style={{ marginBottom: '0.75rem' }}>
                <div className="urgency-bar-track">
                  <div
                    className={`urgency-bar-fill ${triageData.urgency > 7 ? 'high' : triageData.urgency > 4 ? 'medium' : 'low'}`}
                    style={{ width: `${triageData.urgency * 10}%` }}
                  />
                </div>
              </div>
            )}
            <div className="data-row">
              <span className="data-label"><Stethoscope size={10} style={{ display: 'inline', marginRight: '0.3rem', verticalAlign: 'middle' }} />Critical Needs</span>
              <span className="data-value">{triageData.needs || 'N/A'}</span>
            </div>

            {activeItem.type !== 'claim' && triageData.location && triageData.location !== 'N/A' && (
              <iframe
                width="100%"
                height="160"
                style={{ border: 0, borderRadius: '8px', marginTop: '1rem', marginBottom: '1rem', opacity: 0.9 }}
                loading="lazy"
                allowFullScreen
                src={`https://maps.google.com/maps?q=${encodeURIComponent(triageData.location + ' Malaysia')}&z=14&output=embed`}
              ></iframe>
            )}

            <div className="rag-label">
              <FileSearch size={11} /> {activeItem?.type === 'claim' ? 'Vertex AI RAG / Extracted Context' : 'Situation Summary'}
            </div>
            <p className="rag-text">
              {triageData.raw_extraction || 'N/A'}
            </p>
          </div>
        ) : null}
      </div>
    </div>
  );
}
