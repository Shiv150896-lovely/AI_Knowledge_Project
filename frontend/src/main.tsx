import React, { Component, ErrorInfo, FormEvent, ReactNode, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Link, Navigate, NavLink, Route, Routes, useLocation, useNavigate, useParams } from 'react-router-dom'
import './styles.css'

const API = (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '')
const session = {
  get: () => sessionStorage.getItem('access_token') || '',
  set: (token: string) => sessionStorage.setItem('access_token', token),
  clear: () => sessionStorage.removeItem('access_token'),
}

type Doc = { id: string; filename: string; size_bytes: number; page_count: number; status: string; created_at: string }
type BackendDocument = Partial<Doc> & { ID?: string; Filename?: string; SizeBytes?: number; PageCount?: number; Status?: string; CreatedAt?: string }
type BackendOrganization = { name?: string; Name?: string }
type Assistant = { url: string; embed_code: string }
type Usage = { questions_asked: number; answered: number; unable_to_answer: number; human_review: number }
type Reference = { document: string; page: number }
type AskData = { answer: string; outcome: 'answered' | 'unable_to_answer' | 'human_review'; references: Reference[] }

class ApiError extends Error { constructor(public status: number, message: string, public data?: unknown) { super(message) } }
async function request<T>(path: string, init: RequestInit = {}, admin = false): Promise<T> {
  const headers = new Headers(init.headers)
  if (admin) headers.set('Authorization', `Bearer ${session.get()}`)
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const response = await fetch(`${API}${path}`, { ...init, headers })
  const json = await response.json().catch(() => ({}))
  if (!response.ok) throw new ApiError(response.status, json?.error?.message || 'Something went wrong.', json?.data)
  return json.data as T
}
const formatSize = (n: number) => n < 1024 * 1024 ? `${Math.max(1, Math.ceil(n / 1024))} KB` : `${(n / 1024 / 1024).toFixed(1)} MB`
function normalizeDocument(doc: BackendDocument): Doc {
  return { id: doc.id ?? doc.ID ?? '', filename: doc.filename ?? doc.Filename ?? 'Untitled document', size_bytes: doc.size_bytes ?? doc.SizeBytes ?? 0, page_count: doc.page_count ?? doc.PageCount ?? 0, status: doc.status ?? doc.Status ?? 'processing', created_at: doc.created_at ?? doc.CreatedAt ?? new Date().toISOString() }
}
const Icon = ({ name }: { name: string }) => <span className="icon" aria-hidden="true">{name}</span>

function Notice({ kind = 'error', children }: { kind?: 'error' | 'success' | 'info'; children: ReactNode }) { return <div className={`notice ${kind}`} role="status">{children}</div> }
function Brand() { return <Link className="brand" to="/dashboard/knowledge-base"><span className="brand-mark">C</span><span>Clarion<span className="brand-light">Knowledge</span></span></Link> }

function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const navigate = useNavigate(), [error, setError] = useState(''), [busy, setBusy] = useState(false)
  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault(); setError(''); setBusy(true)
    const values = Object.fromEntries(new FormData(e.currentTarget))
    try {
      const data = await request<{ access_token: string }>(mode === 'login' ? '/api/auth/login' : '/api/auth/register', { method: 'POST', body: JSON.stringify(mode === 'login' ? values : values) })
      session.set(data.access_token); navigate('/dashboard/knowledge-base')
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to continue.') } finally { setBusy(false) }
  }
  const registering = mode === 'register'
  return <main className="auth-shell"><section className="auth-intro"><div className="intro-content"><span className="eyebrow light">Knowledge, made clear</span><h1>Your documents.<br/><em>Confident answers.</em></h1><p>Create a private, source-grounded assistant your visitors can trust.</p><div className="intro-points"><span>Tenant-isolated knowledge</span><span>Page-level references</span><span>Decision-aware answers</span></div></div></section><section className="auth-panel"><div className="auth-card"><Link className="brand auth-brand" to={registering ? '/register' : '/login'}><span className="brand-mark">C</span><span>Clarion<span className="brand-light">Knowledge</span></span></Link><span className="eyebrow">{registering ? 'Start your workspace' : 'Welcome back'}</span><h2>{registering ? 'Create your organization' : 'Sign in to your dashboard'}</h2><p className="muted">{registering ? 'Set up your private knowledge assistant in a few steps.' : 'Manage documents, your assistant, and its usage.'}</p>{error && <Notice>{error}</Notice>}<form onSubmit={submit} className="form-stack">{registering && <label>Organization name<input name="organization_name" placeholder="Acme University" required /></label>}<label>Email address<input name="email" type="email" placeholder="you@organization.com" required /></label><label>Password<input name="password" type="password" minLength={8} placeholder="At least 8 characters" required /></label><button className="button primary" disabled={busy}>{busy ? 'Please wait…' : registering ? 'Create organization' : 'Sign in'} <Icon name="→" /></button></form><p className="auth-switch">{registering ? 'Already have an account?' : 'New here?'} <Link to={registering ? '/login' : '/register'}>{registering ? 'Sign in' : 'Create an organization'}</Link></p></div></section></main>
}

function Protected({ children }: { children: ReactNode }) { return session.get() ? <>{children}</> : <Navigate to="/login" replace /> }
function DashboardLayout({ children }: { children: ReactNode }) {
  const nav = useNavigate(), location = useLocation(), [name, setName] = useState('Your organization')
  useEffect(() => { request<BackendOrganization>('/api/organization', {}, true).then(x => setName(x.name ?? x.Name ?? 'Your organization')).catch(err => { if (err instanceof ApiError && err.status === 401) { session.clear(); nav('/login') } }) }, [nav])
  const logout = async () => { try { await request('/api/auth/logout', { method: 'POST' }, true) } catch { /* local logout remains valid */ } session.clear(); nav('/login') }
  const title = location.pathname.includes('assistant') ? 'Assistant' : location.pathname.includes('usage') ? 'Usage' : 'Knowledge base'
  return <div className="dashboard"><aside className="sidebar"><Brand/><div className="workspace"><span className="avatar">{(name || 'O').slice(0, 1).toUpperCase()}</span><span><b>{name || 'Your organization'}</b><small>Organization workspace</small></span></div><nav><NavLink to="/dashboard/knowledge-base"><Icon name="▤"/>Knowledge base</NavLink><NavLink to="/dashboard/assistant"><Icon name="✦"/>Assistant</NavLink><NavLink to="/dashboard/usage"><Icon name="◔"/>Usage</NavLink></nav><button className="logout" onClick={logout}><Icon name="←"/>Log out</button></aside><main className="dashboard-main"><header className="topbar"><div><span className="eyebrow">Workspace</span><h1>{title}</h1></div><span className="secure"><i/> Private to {name || 'your organization'}</span></header>{children}</main></div>
}

function KnowledgeBase() {
  const [docs, setDocs] = useState<Doc[]>([]), [loading, setLoading] = useState(true), [error, setError] = useState(''), [uploading, setUploading] = useState(false), [notice, setNotice] = useState('')
  const load = () => { setLoading(true); request<{ documents: BackendDocument[] }>('/api/documents', {}, true).then(x => setDocs((x.documents || []).map(normalizeDocument))).catch(e => setError(e.message)).finally(() => setLoading(false)) }
  useEffect(() => { void load() }, [])
  const upload = async (file?: File) => { if (!file) return; setError(''); setNotice(''); if (!file.name.toLowerCase().endsWith('.pdf') || file.type && file.type !== 'application/pdf') return setError('Upload a valid PDF document.'); if (file.size > 10 * 1024 * 1024) return setError('This PDF exceeds the 10 MB limit.'); const body = new FormData(); body.append('file', file); setUploading(true); try { await request('/api/documents', { method: 'POST', body }, true); setNotice('Document uploaded and ready for retrieval.'); load() } catch (e) { setError(e instanceof Error ? e.message : 'This document could not be processed. Try another PDF.') } finally { setUploading(false) } }
  const remove = async (doc: Doc) => { if (!confirm(`Delete ${doc.filename}? This cannot be undone.`)) return; setError(''); try { await request(`/api/documents/${doc.id}`, { method: 'DELETE' }, true); setDocs(x => x.filter(d => d.id !== doc.id)); setNotice('Document deleted.') } catch (e) { setError(e instanceof Error ? e.message : 'Could not delete document.') } }
  return <section className="page-content"><div className="page-heading"><div><h2>Build your knowledge base</h2><p>Upload the PDFs your assistant may use to answer visitor questions.</p></div><label className={`button primary ${uploading || docs.length >= 10 ? 'disabled' : ''}`}><input type="file" accept="application/pdf,.pdf" hidden disabled={uploading || docs.length >= 10} onChange={e => upload(e.target.files?.[0])}/>{uploading ? 'Processing PDF…' : <><Icon name="↑"/>Upload PDF</>}</label></div>{error && <Notice>{error}</Notice>}{notice && <Notice kind="success">{notice}</Notice>}<div className="limit-strip"><div><b>{docs.length}</b><span>of 10 documents</span></div><span>PDF only</span><span>Up to 10 MB</span><span>Up to 20 pages</span></div><div className="panel document-panel"><div className="panel-title"><div><h3>Documents</h3><span>Only your organization can access these files.</span></div><span className="count-pill">{docs.length} total</span></div>{loading ? <div className="empty-state">Loading documents…</div> : docs.length === 0 ? <div className="empty-state"><div className="empty-icon">⌁</div><h3>Your knowledge base is empty</h3><p>Upload a PDF to give your assistant grounded information.</p></div> : <div className="doc-list">{docs.map(doc => <div className="doc-row" key={doc.id}><div className="pdf-badge">PDF</div><div className="doc-name"><b>{doc.filename}</b><span>{formatSize(doc.size_bytes)} · {doc.page_count} {doc.page_count === 1 ? 'page' : 'pages'} · {new Date(doc.created_at).toLocaleDateString()}</span></div><span className={`status ${doc.status}`}>{doc.status}</span><button className="icon-button danger" onClick={() => remove(doc)} aria-label={`Delete ${doc.filename}`}>×</button></div>)}</div>}</div></section>
}

function CopyButton({ text, label }: { text: string; label: string }) { const [copied, setCopied] = useState(false); const copy = async () => { await navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 1600) }; return <button className="button secondary small" onClick={copy}>{copied ? 'Copied' : label}</button> }
function AssistantPage() {
  const [assistant, setAssistant] = useState<Assistant | null>(null), [error, setError] = useState(''), [working, setWorking] = useState(false), [showConfirm, setShowConfirm] = useState(false)
  const load = () => request<Assistant>('/api/assistant', {}, true).then(setAssistant).catch(e => setError(e.message)); useEffect(() => { void load() }, [])
  const refresh = async () => { setWorking(true); setError(''); try { setAssistant(await request<Assistant>('/api/assistant/regenerate-token', { method: 'POST' }, true)); setShowConfirm(false) } catch (e) { setError(e instanceof Error ? e.message : 'Could not regenerate assistant.') } finally { setWorking(false) } }
  const token = assistant?.url.split('/').pop() || ''
  return <section className="page-content assistant-page"><div className="page-heading"><div><h2>Share your assistant</h2><p>Visitors can ask questions without signing in. Answers stay grounded in your documents.</p></div></div>{error && <Notice>{error}</Notice>}{!assistant ? <div className="panel empty-state">Loading assistant…</div> : <><div className="assistant-grid"><div className="panel assistant-details"><span className="eyebrow">Your public assistant</span><h3>A private knowledge experience for your visitors.</h3><p className="muted">Your token identifies this assistant without exposing documents or dashboard access.</p><label className="field-label">Assistant URL</label><div className="code-row"><code>{assistant.url}</code><CopyButton text={assistant.url} label="Copy URL"/></div><label className="field-label">Embed code</label><div className="code-row tall"><code>{assistant.embed_code}</code><CopyButton text={assistant.embed_code} label="Copy code"/></div><div className="action-row"><a className="button secondary" href={`/assistant/${token}`} target="_blank" rel="noreferrer">Open assistant <Icon name="↗"/></a><button className="button danger-outline" onClick={() => setShowConfirm(true)}>Regenerate token</button></div></div><div className="panel preview-card"><div className="preview-top"><span>Live preview</span><i/></div><iframe title="Assistant preview" src={`/assistant/${token}`}/></div></div>{showConfirm && <div className="modal-backdrop"><div className="modal"><span className="eyebrow">Confirm action</span><h3>Regenerate the assistant token?</h3><p>Every existing embedded assistant URL will stop working immediately. Copy the new code after regeneration.</p><div className="action-row"><button className="button secondary" onClick={() => setShowConfirm(false)} disabled={working}>Cancel</button><button className="button danger" onClick={refresh} disabled={working}>{working ? 'Regenerating…' : 'Regenerate token'}</button></div></div></div>}</>}</section>
}

function UsagePage() { const [usage, setUsage] = useState<Usage | null>(null), [error, setError] = useState(''); useEffect(() => { request<Usage>('/api/usage', {}, true).then(setUsage).catch(e => setError(e.message)) }, []); const items = usage ? [['Questions asked', usage.questions_asked, '◌'], ['Answered', usage.answered, '✓'], ['Unable to answer', usage.unable_to_answer, '?'], ['Human review', usage.human_review, '↗']] : []; return <section className="page-content"><div className="page-heading"><div><h2>Assistant usage</h2><p>A simple view of how visitors are using your knowledge assistant.</p></div></div>{error && <Notice>{error}</Notice>}{!usage ? <div className="panel empty-state">Loading usage…</div> : <div className="usage-grid">{items.map(([label, value, icon]) => <div className="usage-card" key={String(label)}><span className="usage-icon">{icon}</span><span>{label}</span><strong>{value}</strong></div>)}</div>}</section> }

function PublicAssistant() {
  const { assistantToken = '' } = useParams(), [messages, setMessages] = useState<{ role: 'user' | 'assistant'; text: string; references?: Reference[]; outcome?: string }[]>([{ role: 'assistant', text: 'Hello. Ask a question about this organization’s documents.' }]), [question, setQuestion] = useState(''), [busy, setBusy] = useState(false)
  const ask = async (e: FormEvent) => { e.preventDefault(); const q = question.trim(); if (!q || busy) return; setMessages(x => [...x, { role: 'user', text: q }]); setQuestion(''); setBusy(true); try { const response = await fetch(`${API}/api/ask`, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Assistant-Token': assistantToken }, body: JSON.stringify({ question: q }) }); const json = await response.json().catch(() => ({})); const data: AskData | undefined = json.data; if (data && (response.ok || response.status === 422)) { setMessages(x => [...x, { role: 'assistant', text: data.answer, references: data.references, outcome: data.outcome }]) } else { setMessages(x => [...x, { role: 'assistant', text: json?.error?.message || 'The assistant could not process this question. Please try again.' }]) } } catch { setMessages(x => [...x, { role: 'assistant', text: 'The assistant could not process this question. Please try again.' }]) } finally { setBusy(false) } }
  return <main className="chat-shell"><header className="chat-header"><div className="chat-logo"><span className="brand-mark">C</span><span><b>Knowledge assistant</b><small><i/>Grounded in organization documents</small></span></div></header><section className="conversation">{messages.map((m, i) => <div className={`message ${m.role}`} key={i}><div className="bubble">{m.text}{m.references && m.references.length > 0 && <div className="sources"><span>Sources</span>{m.references.map((r, ix) => <small key={ix}>{r.document} — Page {r.page}</small>)}</div>}</div></div>)}{busy && <div className="message assistant"><div className="bubble typing"><i/><i/><i/></div></div>}</section><form className="chat-form" onSubmit={ask}><input value={question} onChange={e => setQuestion(e.target.value)} placeholder="Ask a question…" aria-label="Question" disabled={busy}/><button className="send" disabled={busy || !question.trim()} aria-label="Send question">↑</button></form><footer className="chat-footer">Answers are grounded in this organization’s uploaded documents.</footer></main>
}

function App() { return <Routes><Route path="/login" element={<AuthPage mode="login"/>}/><Route path="/register" element={<AuthPage mode="register"/>}/><Route path="/assistant/:assistantToken" element={<PublicAssistant/>}/><Route path="/dashboard/knowledge-base" element={<Protected><DashboardLayout><KnowledgeBase/></DashboardLayout></Protected>}/><Route path="/dashboard/assistant" element={<Protected><DashboardLayout><AssistantPage/></DashboardLayout></Protected>}/><Route path="/dashboard/usage" element={<Protected><DashboardLayout><UsagePage/></DashboardLayout></Protected>}/><Route path="*" element={<Navigate to={session.get() ? '/dashboard/knowledge-base' : '/login'} replace/>}/></Routes> }
class AppErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }
  static getDerivedStateFromError(error: Error) { return { error } }
  componentDidCatch(error: Error, info: ErrorInfo) { console.error('Frontend render error', error, info) }
  render() { return this.state.error ? <main className="fatal-error"><div><span className="brand-mark">C</span><h1>The dashboard could not load.</h1><p>{this.state.error.message}</p><button className="button primary" onClick={() => window.location.assign('/login')}>Return to sign in</button></div></main> : this.props.children }
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><AppErrorBoundary><BrowserRouter><App/></BrowserRouter></AppErrorBoundary></React.StrictMode>)
