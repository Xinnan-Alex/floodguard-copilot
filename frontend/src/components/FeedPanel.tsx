import { useRef } from 'react';
import type { FeedItem } from '../types';
import { Radio, Phone, MessageCircle, FileText, ImagePlus } from 'lucide-react';

interface FeedPanelProps {
  feeds: FeedItem[];
  activeId: string | null;
  onSelect: (item: FeedItem) => void;
  onUpload: (file: File) => void;
}

export default function FeedPanel({ feeds, activeId, onSelect, onUpload }: FeedPanelProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files?.[0]) {
      onUpload(e.target.files[0]);
    }
  };

  return (
    <div className="glass-panel">
      <div className="panel-title">
        <Radio size={13} /> Live Intake Stream
      </div>

      <div style={{ marginBottom: '1rem' }}>
        <input
          type="file"
          accept="image/*"
          style={{ display: 'none' }}
          ref={fileInputRef}
          onChange={handleFileChange}
        />
        <button
          className="btn btn-ingest"
          onClick={() => fileInputRef.current?.click()}
        >
          <ImagePlus size={14} /> Ingest Flood Photo
        </button>
      </div>

      <div className="feed-list">
        {feeds.map((feed) => (
          <div
            key={feed.id}
            className={`feed-item ${feed.type} ${activeId === feed.id ? 'active' : ''}`}
            onClick={() => onSelect(feed)}
          >
            <div className={`feed-badge ${feed.type}`}>
              {feed.type === 'call' && <Phone size={9} />}
              {feed.type === 'whatsapp' && <MessageCircle size={9} />}
              {feed.type === 'claim' && <FileText size={9} />}
              {feed.type}
            </div>
            <h4>{feed.preview}</h4>
            <p className="feed-item-time">{feed.timestamp}</p>
            {feed.imageObjectUrl && (
              <img
                src={feed.imageObjectUrl}
                alt="Uploaded feed"
                style={{ marginTop: '0.5rem', width: '100%', height: '80px', objectFit: 'cover', borderRadius: '4px' }}
              />
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
