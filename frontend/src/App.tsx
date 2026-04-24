import { useState, useEffect, useRef, useCallback } from 'react'
import { Shield, Clock } from 'lucide-react'
import ThemeToggle from './components/ThemeToggle'
import './App.css'
import type { DispatchResult, FeedItem, TriageResult } from './types'
import { backendUrl, postJSON } from './api'
import FeedPanel from './components/FeedPanel'
import TriagePanel from './components/TriagePanel'
import ActionPanel from './components/ActionPanel'

const DISPATCH_RESULTS_STORAGE_KEY = 'dispatchResultsByFeedId';

const DEMO_FEEDS: FeedItem[] = [
  { id: '1', type: 'call', timestamp: '10:42 AM', preview: 'Emergency Call - Shah Alam', fullData: 'Operator: 999 what is your emergency?\nCaller: Water is entering our house in Section 13 Shah Alam, it is knee deep and my grandfather is bedridden. We need help to evacuate.' },
  { id: '2', type: 'whatsapp', timestamp: '10:45 AM', preview: 'WhatsApp Message - Klang', fullData: 'WhatsApp Alert: Send coordinate and conditions near Klang town.' },
  { id: '3', type: 'claim', timestamp: '10:50 AM', preview: 'Relief Claim - IC verification', fullData: 'Victim Ahmad bin Abu, IC 900101-10-1234, house destroyed in Hulu Langat.' },
];

function App() {
  const [activeItem, setActiveItem] = useState<FeedItem | null>(null);
  const [loading, setLoading] = useState(false);
  const [triageData, setTriageData] = useState<TriageResult | null>(null);
  const [feeds, setFeeds] = useState<FeedItem[]>(DEMO_FEEDS);
  const triageCacheRef = useRef<Record<string, TriageResult>>({});
  const lastFeedIdRef = useRef<string>('');
  const [dispatchResultsById, setDispatchResultsById] = useState<Record<string, DispatchResult>>(() => {
    try {
      const raw = localStorage.getItem(DISPATCH_RESULTS_STORAGE_KEY);
      if (!raw) return {};
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') {
        return parsed as Record<string, DispatchResult>;
      }
    } catch {
      // Ignore malformed persisted state and start fresh.
    }
    return {};
  });

  // Poll backend for new WhatsApp messages every 3 seconds
  useEffect(() => {
    const interval = setInterval(async () => {
      try {
        const sinceParam = lastFeedIdRef.current ? `?since=${lastFeedIdRef.current}` : '';
        const res = await fetch(backendUrl(`/api/feeds${sinceParam}`));
        if (res.ok) {
          const newItems: FeedItem[] = await res.json();
          if (newItems?.length > 0) {
            lastFeedIdRef.current = newItems[0].id;
            setFeeds(prev => {
              const existingIds = new Set(prev.map(f => f.id));
              const uniqueNew = newItems.filter(item => !existingIds.has(item.id));
              return [...uniqueNew, ...prev];
            });
          }
        }
      } catch {
        // Backend offline — silent fail, demo feeds still visible
      }
    }, 3000);
    return () => clearInterval(interval);
  }, []);

  const processItem = useCallback(async (item: FeedItem) => {
    setLoading(true);
    setTriageData(null);
    try {
      const endpoint = item.type === 'call' ? '/api/whisper'
        : item.type === 'whatsapp' ? '/api/triage'
        : '/api/claim';

      const payload: Record<string, unknown> = { transcript: item.fullData };
      if (endpoint === '/api/triage' && item.imageBase64) {
        payload.image_base64 = item.imageBase64;
        payload.image_mime_type = item.imageMimeType ?? 'image/jpeg';
      }

      const data = await postJSON<TriageResult>(endpoint, payload);

      const result: TriageResult = {
        ...data,
        location: data.location || 'Unknown',
        urgency: data.urgency || 5,
        needs: data.needs || 'Analysis pending',
        status: dispatchResultsById[item.id] ? 'dispatched' : (data.status || 'pending_approval'),
        raw_extraction: data.raw_extraction || data.reasoning || item.fullData,
      };
      triageCacheRef.current[item.id] = result;
      setTriageData(result);
    } catch (e) {
      console.error(e);
      const fallback: TriageResult = {
        location: item.type === 'whatsapp' ? 'Klang' : 'Shah Alam',
        urgency: item.type === 'whatsapp' ? 9 : 8,
        needs: 'Evacuation',
        status: dispatchResultsById[item.id] ? 'dispatched' : 'pending_approval',
        raw_extraction: item.fullData,
      };
      triageCacheRef.current[item.id] = fallback;
      setTriageData(fallback);
    } finally {
      setLoading(false);
    }
  }, [dispatchResultsById]);

  useEffect(() => {
    if (activeItem) {
      const cached = triageCacheRef.current[activeItem.id];
      if (cached) {
        const enforced = dispatchResultsById[activeItem.id] && cached.status !== 'dispatched'
          ? { ...cached, status: 'dispatched' }
          : cached;
        if (enforced !== cached) {
          triageCacheRef.current[activeItem.id] = enforced;
        }
        setTriageData(enforced);
      } else {
        processItem(activeItem);
      }
    }
  }, [activeItem, processItem, dispatchResultsById]);

  useEffect(() => {
    localStorage.setItem(DISPATCH_RESULTS_STORAGE_KEY, JSON.stringify(dispatchResultsById));
  }, [dispatchResultsById]);

  const fileToBase64 = (file: File): Promise<string> => new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const value = typeof reader.result === 'string' ? reader.result : '';
      const base64 = value.includes(',') ? value.split(',')[1] : value;
      if (base64) {
        resolve(base64);
        return;
      }
      reject(new Error('Failed to read file as base64'));
    };
    reader.onerror = () => reject(reader.error ?? new Error('File read failed'));
    reader.readAsDataURL(file);
  });

  const handleUpload = async (file: File) => {
    const objectUrl = URL.createObjectURL(file);
    let imageBase64 = '';
    try {
      imageBase64 = await fileToBase64(file);
    } catch (e) {
      console.error(e);
    }

    const newItem: FeedItem = {
      id: Date.now().toString(),
      type: 'whatsapp',
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      preview: 'Uploaded Image Triage',
      fullData: `[IMAGE UPLOADED]: Filename ${file.name}. Requesting Gemini Pro Vision analysis.`,
      imageObjectUrl: objectUrl,
      imageBase64,
      imageMimeType: file.type || 'image/jpeg',
    };
    setFeeds(prev => [newItem, ...prev]);
    setActiveItem(newItem);
  };

  const handleStatusChange = useCallback((itemId: string, status: string) => {
    const cached = triageCacheRef.current[itemId];
    if (cached) {
      triageCacheRef.current[itemId] = { ...cached, status };
    }
    setTriageData(prev => (activeItem?.id === itemId && prev ? { ...prev, status } : prev));
  }, [activeItem?.id]);

  const handleDispatchComplete = useCallback((itemId: string, result: DispatchResult) => {
    setDispatchResultsById(prev => ({ ...prev, [itemId]: result }));
  }, []);

  const activeDispatchResult = activeItem ? dispatchResultsById[activeItem.id] ?? null : null;
  const activeIsDispatched = Boolean(activeDispatchResult);

  // Lazy init from localStorage (vercel-react: rerender-lazy-state-init)
  const [theme, setTheme] = useState<'dark' | 'light'>(
    () => (localStorage.getItem('theme') as 'dark' | 'light') ?? 'dark'
  );

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
  }, [theme]);

  // Functional setState for stable toggle (vercel-react: rerender-functional-setstate)
  const toggleTheme = useCallback(() => setTheme(t => t === 'dark' ? 'light' : 'dark'), []);

  const [clock, setClock] = useState(() =>
    new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  );
  useEffect(() => {
    const t = setInterval(() =>
      setClock(new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }))
    , 1000);
    return () => clearInterval(t);
  }, []);

  return (
    <>
    <header className="app-header">
      <div className="app-header-left">
        <div className="app-logo-icon">
          <Shield size={16} />
        </div>
        <div>
          <div className="app-title">FloodGuard Copilot</div>
          <div className="app-subtitle">Emergency Dispatch AI</div>
        </div>
      </div>
      <div className="app-header-right">
        <div className="live-indicator">
          <span className="live-dot" />
          Live
        </div>
        <div className="header-time">
          <Clock size={11} />
          {clock}
        </div>
        <ThemeToggle theme={theme} onToggle={toggleTheme} />
      </div>
    </header>
    <div className="dashboard-container">
      <FeedPanel
        feeds={feeds}
        activeId={activeItem?.id ?? null}
        onSelect={setActiveItem}
        onUpload={handleUpload}
      />
      <TriagePanel
        activeItem={activeItem}
        loading={loading}
        triageData={triageData}
      />
      <ActionPanel
        activeItem={activeItem}
        loading={loading}
        triageData={triageData}
        dispatchResult={activeDispatchResult}
        isDispatched={activeIsDispatched}
        onDispatchComplete={handleDispatchComplete}
        onStatusChange={handleStatusChange}
      />
    </div>
    </>
  )
}

export default App
