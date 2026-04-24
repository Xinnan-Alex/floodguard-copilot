import { useState } from 'react';
import type { FeedItem, TriageResult, DispatchResult } from '../types';
import { postJSON } from '../api';
import { Zap, CheckCircle2, Loader2, Navigation, Clock} from 'lucide-react';

interface ActionPanelProps {
  activeItem: FeedItem | null;
  loading: boolean;
  triageData: TriageResult | null;
  dispatchResult: DispatchResult | null;
  isDispatched: boolean;
  onDispatchComplete: (itemId: string, result: DispatchResult) => void;
  onStatusChange: (itemId: string, status: string) => void;
}

export default function ActionPanel({
  activeItem,
  loading,
  triageData,
  dispatchResult,
  isDispatched,
  onDispatchComplete,
  onStatusChange,
}: ActionPanelProps) {
  const [dispatching, setDispatching] = useState(false);
  const [dispatchError, setDispatchError] = useState<string | null>(null);

  const handleAction = async () => {
    if (!triageData || !activeItem) return;
    setDispatching(true);
    setDispatchError(null);

    try {
      const data = await postJSON<DispatchResult>('/api/dispatch', {
        action_type: activeItem.type === 'claim' ? 'claim_filing' : 'rescue_dispatch',
        location: triageData.location || 'Unknown',
        urgency: triageData.urgency || 5,
        needs: triageData.needs || 'General',
        amount: triageData.amount || 0,
        reasoning: triageData.reasoning || '',
      });
      onDispatchComplete(activeItem.id, data);
      onStatusChange(activeItem.id, 'dispatched');
    } catch (e) {
      console.error(e);
      setDispatchError('Dispatch failed. Please retry.');
    } finally {
      setDispatching(false);
    }
  };

  return (
    <div className="glass-panel">
      <div className="panel-title">
        <Zap size={13} /> Action Orchestrator
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', flex: 1 }}>
        <p style={{ color: 'var(--text-muted)', marginBottom: '1rem', fontSize: '0.8rem', lineHeight: '1.5' }}>
          The Agent has formulated the following autonomous actions based on the intelligence report. Human approval is required to initiate execution.
        </p>

        <div className="action-box">
          <strong>Proposed Agentic Action</strong>
          {activeItem?.type === 'claim'
            ? (loading ? 'Calculating eligibility...' : `API: ${triageData?.status === 'approved' ? 'Submit' : 'Review'} RM${triageData?.amount || 1000} Relief Form (${triageData?.reasoning || 'NADMA Policy'})`)
            : (loading ? 'Determining dispatch route...' : `Dispatch: Route Rescue Boat to ${triageData?.location || 'identified coordinates'}`)}
        </div>

        <button
          className={`btn ${isDispatched ? 'btn-success' : ''}`}
          onClick={handleAction}
          disabled={!triageData || isDispatched || dispatching}
          style={{ marginTop: 'auto' }}
        >
          {dispatching
            ? <><Loader2 size={14} style={{ animation: 'spin 1s linear infinite' }} /> Executing…</>
            : isDispatched
              ? <><CheckCircle2 size={14} /> Action Executed</>
              : <><Zap size={14} /> Approve & Execute Workflow</>}
        </button>

        {dispatchError && (
          <p style={{ color: 'var(--danger)', fontSize: '0.8rem', marginTop: '0.5rem' }}>{dispatchError}</p>
        )}

        {dispatchResult && (
          <div className="dispatch-card">
            <div className="dispatch-card-header">
              <CheckCircle2 size={14} /> Dispatch Confirmed
            </div>
            <div className="dispatch-id">{dispatchResult.confirmation_id}</div>
            <div className="dispatch-meta-row">
              <span className="dispatch-meta-label"><Navigation size={10} /> Routed To</span>
              <span className="dispatch-meta-value">{dispatchResult.routed_to}</span>
            </div>
            {dispatchResult.eta_minutes !== undefined && dispatchResult.eta_minutes > 0 && (
              <div className="dispatch-meta-row">
                <span className="dispatch-meta-label"><Clock size={10} /> ETA</span>
                <span className="dispatch-meta-value">{dispatchResult.eta_minutes} min</span>
              </div>
            )}
            {dispatchResult.summary && <p className="dispatch-summary">{dispatchResult.summary}</p>}
          </div>
        )}
      </div>
    </div>
  );
}
