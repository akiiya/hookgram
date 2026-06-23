import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  Activity,
  Bot,
  CheckCircle2,
  Copy,
  Eye,
  KeyRound,
  LockKeyhole,
  LogOut,
  Pencil,
  Plus,
  RefreshCcw,
  Save,
  Send,
  Settings,
  Shield,
  Trash2,
  UserRound,
  Users,
  XCircle
} from 'lucide-react';
import './styles.css';

type AdminUser = {
  id: number;
  username: string;
  last_login_at?: string | null;
};

type TelegramUser = {
  id: number;
  telegram_user_id: number;
  chat_id: number;
  username: string;
  display_name: string;
  status: string;
  last_seen_at?: string | null;
  token_count?: number;
  message_count?: number;
};

type WebhookToken = {
  id: number;
  alias: string;
  token_prefix: string;
  use_count: number;
  last_used_at?: string | null;
  disabled_at?: string | null;
  created_at: string;
};

type WebhookMessage = {
  id: number;
  token?: WebhookToken;
  title: string;
  content: string;
  raw_payload: string;
  format: string;
  level: string;
  source: string;
  source_ip: string;
  user_agent: string;
  delivery_status: string;
  delivery_error: string;
  telegram_message_id: number;
  created_at: string;
};

type VersionInfo = {
  version: string;
  commit: string;
  buildDate: string;
  platform: string;
};

type ApiError = Error & { status?: number };

async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {})
    },
    ...options
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const err = new Error(data.error || '请求失败') as ApiError;
    err.status = res.status;
    throw err;
  }
  return data as T;
}

function usePath() {
  const [path, setPath] = useState(window.location.pathname);
  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);
  const navigate = (to: string) => {
    window.history.pushState({}, '', to);
    setPath(to);
  };
  return { path, navigate };
}

function App() {
  const { path, navigate } = usePath();
  const [initialized, setInitialized] = useState<boolean | null>(null);
  const [admin, setAdmin] = useState<AdminUser | null>(null);
  const [loading, setLoading] = useState(true);

  const refreshAuth = async () => {
    const setup = await api<{ initialized: boolean }>('/api/setup/status');
    setInitialized(setup.initialized);
    if (!setup.initialized) {
      setAdmin(null);
      return;
    }
    try {
      const me = await api<{ admin: AdminUser }>('/api/auth/me');
      setAdmin(me.admin);
    } catch {
      setAdmin(null);
    }
  };

  useEffect(() => {
    refreshAuth().finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (initialized === false && path !== '/setup') navigate('/setup');
    if (initialized === true && path === '/setup') navigate('/login');
    if (initialized === true && path === '/') navigate('/admin');
  }, [initialized, path]);

  if (loading || initialized === null) return <FullScreenStatus text="Hookgram 正在启动" />;
  if (!initialized) return <SetupPage onDone={() => { setInitialized(true); navigate('/login'); }} />;
  if (path === '/login' || !admin) return <LoginPage onLogin={(next) => { setAdmin(next); navigate('/admin'); }} />;

  return (
    <AdminLayout admin={admin} path={path} navigate={navigate} onLogout={() => setAdmin(null)}>
      <RouteView path={path} navigate={navigate} onPasswordChanged={() => setAdmin(null)} />
    </AdminLayout>
  );
}

function FullScreenStatus({ text }: { text: string }) {
  return (
    <div className="min-h-screen bg-neutral-100 flex items-center justify-center text-neutral-700">
      <div className="flex items-center gap-3 rounded-lg bg-white px-5 py-4 shadow-soft">
        <RefreshCcw className="h-5 w-5 animate-spin text-teal-600" />
        <span>{text}</span>
      </div>
    </div>
  );
}

function SetupPage({ onDone }: { onDone: () => void }) {
  const [form, setForm] = useState({ username: 'admin', password: '', bot_token: '', api_proxy: '', base_url: '' });
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSaving(true);
    try {
      await api('/api/setup', { method: 'POST', body: JSON.stringify(form) });
      onDone();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  };
  return (
    <AuthShell title="初始化 Hookgram" subtitle="创建管理员并填写 Telegram Bot 基础配置">
      <form className="space-y-4" onSubmit={submit}>
        <Input label="管理员账号" value={form.username} onChange={(v) => setForm({ ...form, username: v })} />
        <Input label="管理员密码" type="password" value={form.password} onChange={(v) => setForm({ ...form, password: v })} />
        <Input label="Telegram Bot Token" value={form.bot_token} onChange={(v) => setForm({ ...form, bot_token: v })} />
        <Input label="Telegram API Proxy" value={form.api_proxy} placeholder="可留空" onChange={(v) => setForm({ ...form, api_proxy: v })} />
        <Input label="Base URL" value={form.base_url} placeholder="可留空" onChange={(v) => setForm({ ...form, base_url: v })} />
        {error && <Alert kind="error" text={error} />}
        <Button type="submit" icon={<Shield />} disabled={saving}>{saving ? '正在初始化' : '完成初始化'}</Button>
      </form>
    </AuthShell>
  );
}

function LoginPage({ onLogin }: { onLogin: (admin: AdminUser) => void }) {
  const [form, setForm] = useState({ username: 'admin', password: '' });
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSaving(true);
    try {
      const res = await api<{ admin: AdminUser }>('/api/auth/login', { method: 'POST', body: JSON.stringify(form) });
      onLogin(res.admin);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  };
  return (
    <AuthShell title="登录 Hookgram" subtitle="进入自托管 Webhook 消息转发系统">
      <form className="space-y-4" onSubmit={submit}>
        <Input label="管理员账号" value={form.username} onChange={(v) => setForm({ ...form, username: v })} />
        <Input label="管理员密码" type="password" value={form.password} onChange={(v) => setForm({ ...form, password: v })} />
        {error && <Alert kind="error" text={error} />}
        <Button type="submit" icon={<LockKeyhole />} disabled={saving}>{saving ? '正在登录' : '登录'}</Button>
      </form>
    </AuthShell>
  );
}

function AuthShell({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-[linear-gradient(135deg,#f7f7f5_0%,#eef7f4_42%,#f8f1ef_100%)] flex items-center justify-center p-5">
      <div className="w-full max-w-md rounded-lg bg-white p-8 shadow-soft border border-neutral-200">
        <div className="mb-7">
          <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-lg bg-teal-600 text-white">
            <Bot className="h-6 w-6" />
          </div>
          <h1 className="text-2xl font-semibold text-neutral-950">{title}</h1>
          <p className="mt-2 text-sm text-neutral-500">{subtitle}</p>
        </div>
        {children}
      </div>
    </div>
  );
}

function AdminLayout({ admin, path, navigate, onLogout, children }: {
  admin: AdminUser;
  path: string;
  navigate: (to: string) => void;
  onLogout: () => void;
  children: React.ReactNode;
}) {
  const nav = [
    { path: '/admin', label: '总览', icon: Activity },
    { path: '/admin/users', label: 'Bot 用户', icon: Users },
    { path: '/admin/settings', label: '系统设置', icon: Settings },
    { path: '/admin/profile', label: '管理员', icon: UserRound }
  ];
  const logout = async () => {
    await api('/api/auth/logout', { method: 'POST', body: '{}' }).catch(() => null);
    onLogout();
    navigate('/login');
  };
  return (
    <div className="min-h-screen bg-neutral-100 text-neutral-950">
      <aside className="fixed inset-y-0 left-0 hidden w-64 border-r border-neutral-200 bg-white lg:block">
        <div className="flex h-16 items-center gap-3 border-b border-neutral-200 px-6">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-teal-600 text-white"><Bot className="h-5 w-5" /></div>
          <div>
            <div className="font-semibold">Hookgram</div>
            <div className="text-xs text-neutral-500">Webhook Hub</div>
          </div>
        </div>
        <nav className="space-y-1 p-3">
          {nav.map((item) => {
            const active = path === item.path || (item.path !== '/admin' && path.startsWith(item.path));
            const Icon = item.icon;
            return (
              <button key={item.path} onClick={() => navigate(item.path)} className={`nav-item ${active ? 'nav-active' : ''}`}>
                <Icon className="h-4 w-4" />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>
      </aside>
      <div className="lg:pl-64">
        <header className="sticky top-0 z-10 flex h-16 items-center justify-between border-b border-neutral-200 bg-white/90 px-4 backdrop-blur lg:px-8">
          <div className="flex items-center gap-3 lg:hidden">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-teal-600 text-white"><Bot className="h-5 w-5" /></div>
            <span className="font-semibold">Hookgram</span>
          </div>
          <div className="hidden text-sm text-neutral-500 lg:block">管理员：{admin.username}</div>
          <div className="flex items-center gap-2">
            <button className="icon-btn" title="退出登录" onClick={logout}><LogOut className="h-4 w-4" /></button>
          </div>
        </header>
        <main className="mx-auto max-w-7xl p-4 lg:p-8">{children}</main>
      </div>
    </div>
  );
}

function RouteView({ path, navigate, onPasswordChanged }: { path: string; navigate: (to: string) => void; onPasswordChanged: () => void }) {
  const userMatch = path.match(/^\/admin\/users\/(\d+)/);
  if (userMatch) return <UserDetailPage userId={Number(userMatch[1])} />;
  if (path === '/admin/users') return <UsersPage navigate={navigate} />;
  if (path === '/admin/settings') return <SettingsPage />;
  if (path === '/admin/profile') return <ProfilePage onPasswordChanged={onPasswordChanged} />;
  return <DashboardPage navigate={navigate} />;
}

function DashboardPage({ navigate }: { navigate: (to: string) => void }) {
  const [data, setData] = useState<any>(null);
  useEffect(() => { api('/api/admin/dashboard').then(setData); }, []);
  if (!data) return <PageLoading />;
  const metrics = [
    { label: 'Bot 用户', value: data.bot_users, icon: Users, color: 'text-teal-700' },
    { label: 'Token', value: data.tokens, icon: KeyRound, color: 'text-violet-700' },
    { label: '今日推送', value: data.today_messages, icon: Send, color: 'text-amber-700' },
    { label: '发送成功', value: data.sent, icon: CheckCircle2, color: 'text-emerald-700' },
    { label: '发送失败', value: data.failed, icon: XCircle, color: 'text-rose-700' }
  ];
  return (
    <div className="space-y-6">
      <PageTitle title="总览" desc="Bot 用户、Token 和 Webhook 推送状态" />
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        {metrics.map((metric) => {
          const Icon = metric.icon;
          return (
            <div key={metric.label} className="rounded-lg border border-neutral-200 bg-white p-5 shadow-sm">
              <div className={`mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-neutral-100 ${metric.color}`}><Icon className="h-5 w-5" /></div>
              <div className="text-2xl font-semibold">{metric.value}</div>
              <div className="mt-1 text-sm text-neutral-500">{metric.label}</div>
            </div>
          );
        })}
      </div>
      <Section title="最近推送" action={<button className="text-sm text-teal-700" onClick={() => navigate('/admin/users')}>查看用户</button>}>
        <MessageTable items={data.recent || []} />
      </Section>
    </div>
  );
}

function UsersPage({ navigate }: { navigate: (to: string) => void }) {
  const [items, setItems] = useState<TelegramUser[]>([]);
  useEffect(() => { api<{ items: TelegramUser[] }>('/api/admin/users').then((res) => setItems(res.items)); }, []);
  return (
    <div className="space-y-6">
      <PageTitle title="Bot 用户" desc="与 Telegram Bot 交互过的用户" />
      <Section title="用户列表">
        <div className="overflow-x-auto">
          <table className="table">
            <thead><tr><th>显示名</th><th>Username</th><th>Chat ID</th><th>状态</th><th>Token</th><th>推送</th><th>最近活跃</th><th>操作</th></tr></thead>
            <tbody>
              {items.map((u) => (
                <tr key={u.id}>
                  <td className="font-medium">{u.display_name || '-'}</td>
                  <td>{u.username ? `@${u.username}` : '-'}</td>
                  <td>{u.chat_id}</td>
                  <td><StatusPill value={u.status} /></td>
                  <td>{u.token_count || 0}</td>
                  <td>{u.message_count || 0}</td>
                  <td>{formatDate(u.last_seen_at)}</td>
                  <td><button className="icon-btn" title="管理" onClick={() => navigate(`/admin/users/${u.id}`)}><Eye className="h-4 w-4" /></button></td>
                </tr>
              ))}
              {items.length === 0 && <EmptyRow colSpan={8} text="暂无 Bot 用户，用户发送 /start 后会出现在这里。" />}
            </tbody>
          </table>
        </div>
      </Section>
    </div>
  );
}

function UserDetailPage({ userId }: { userId: number }) {
  const [user, setUser] = useState<TelegramUser | null>(null);
  const [tab, setTab] = useState<'tokens' | 'messages'>('tokens');
  const load = () => api<{ user: TelegramUser }>(`/api/admin/users/${userId}`).then((res) => setUser(res.user));
  useEffect(() => { load(); }, [userId]);
  if (!user) return <PageLoading />;
  return (
    <div className="space-y-6">
      <PageTitle title={user.display_name || 'Bot 用户'} desc={`${user.username ? '@' + user.username + ' · ' : ''}Chat ID ${user.chat_id}`} />
      <div className="flex gap-2">
        <button className={`tab-btn ${tab === 'tokens' ? 'tab-active' : ''}`} onClick={() => setTab('tokens')}>Webhook Token</button>
        <button className={`tab-btn ${tab === 'messages' ? 'tab-active' : ''}`} onClick={() => setTab('messages')}>推送记录</button>
      </div>
      {tab === 'tokens' ? <TokenPanel userId={userId} /> : <UserMessagesPanel userId={userId} />}
    </div>
  );
}

function TokenPanel({ userId }: { userId: number }) {
  const [items, setItems] = useState<WebhookToken[]>([]);
  const [alias, setAlias] = useState('');
  const [created, setCreated] = useState<any>(null);
  const [error, setError] = useState('');
  const load = () => api<{ items: WebhookToken[] }>(`/api/admin/users/${userId}/tokens`).then((res) => setItems(res.items));
  useEffect(() => { load(); }, [userId]);
  const create = async () => {
    setError('');
    try {
      const res = await api(`/api/admin/users/${userId}/tokens`, { method: 'POST', body: JSON.stringify({ alias }) });
      setAlias('');
      setCreated(res);
      load();
    } catch (err) {
      setError((err as Error).message);
    }
  };
  const rename = async (token: WebhookToken) => {
    const next = window.prompt('新的 Token 别名', token.alias);
    if (!next || next === token.alias) return;
    await api(`/api/admin/users/${userId}/tokens/${token.id}`, { method: 'PATCH', body: JSON.stringify({ alias: next }) });
    load();
  };
  const toggle = async (token: WebhookToken) => {
    await api(`/api/admin/users/${userId}/tokens/${token.id}`, { method: 'PATCH', body: JSON.stringify({ disabled: !token.disabled_at }) });
    load();
  };
  const remove = async (token: WebhookToken) => {
    if (!window.confirm(`确认删除 Token「${token.alias}」？此操作不可恢复。`)) return;
    await api(`/api/admin/users/${userId}/tokens/${token.id}`, { method: 'DELETE' });
    load();
  };
  return (
    <Section title="Webhook Token" action={<Button icon={<Plus />} onClick={create}>创建 Token</Button>}>
      <div className="mb-4 flex flex-col gap-3 sm:flex-row">
        <input className="input flex-1" value={alias} onChange={(e) => setAlias(e.target.value)} placeholder="别名，可留空自动生成" />
      </div>
      {error && <Alert kind="error" text={error} />}
      {created && <OneTimeToken data={created} onClose={() => setCreated(null)} />}
      <div className="overflow-x-auto">
        <table className="table">
          <thead><tr><th>别名</th><th>前缀</th><th>创建时间</th><th>使用次数</th><th>最近使用</th><th>状态</th><th>操作</th></tr></thead>
          <tbody>
            {items.map((token) => (
              <tr key={token.id}>
                <td className="font-medium">{token.alias}</td>
                <td>{token.token_prefix}</td>
                <td>{formatDate(token.created_at)}</td>
                <td>{token.use_count}</td>
                <td>{formatDate(token.last_used_at)}</td>
                <td><StatusPill value={token.disabled_at ? 'disabled' : 'active'} /></td>
                <td className="flex gap-2">
                  <button className="icon-btn" title="修改别名" onClick={() => rename(token)}><Pencil className="h-4 w-4" /></button>
                  <button className="icon-btn" title={token.disabled_at ? '启用' : '禁用'} onClick={() => toggle(token)}><KeyRound className="h-4 w-4" /></button>
                  <button className="icon-btn danger" title="删除" onClick={() => remove(token)}><Trash2 className="h-4 w-4" /></button>
                </td>
              </tr>
            ))}
            {items.length === 0 && <EmptyRow colSpan={7} text="暂无 Token。" />}
          </tbody>
        </table>
      </div>
    </Section>
  );
}

function OneTimeToken({ data, onClose }: { data: any; onClose: () => void }) {
  const token = data.plain_token;
  const url = data.webhook_url;
  return (
    <div className="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm">
      <div className="mb-2 font-semibold text-emerald-800">Token 创建成功，仅显示一次</div>
      <div className="space-y-2 font-mono text-xs text-neutral-800">
        <CopyLine text={token} />
        <CopyLine text={url} />
      </div>
      <button className="mt-3 text-sm text-emerald-800" onClick={onClose}>我已保存</button>
    </div>
  );
}

function CopyLine({ text }: { text: string }) {
  return (
    <div className="flex items-center gap-2 rounded bg-white px-3 py-2">
      <span className="min-w-0 flex-1 break-all">{text}</span>
      <button className="icon-btn" title="复制" onClick={() => navigator.clipboard?.writeText(text)}><Copy className="h-4 w-4" /></button>
    </div>
  );
}

function UserMessagesPanel({ userId }: { userId: number }) {
  const [items, setItems] = useState<WebhookMessage[]>([]);
  useEffect(() => { api<{ items: WebhookMessage[] }>(`/api/admin/users/${userId}/messages`).then((res) => setItems(res.items)); }, [userId]);
  return <Section title="Webhook 推送记录"><MessageTable items={items} /></Section>;
}

function SettingsPage() {
  const [settings, setSettings] = useState<any>(null);
  const [version, setVersion] = useState<VersionInfo | null>(null);
  const [form, setForm] = useState({ base_url: '', bot_token: '', api_proxy: '' });
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const load = async () => {
    const [settingsRes, versionRes] = await Promise.all([
      api<any>('/api/admin/settings'),
      api<VersionInfo>('/api/version')
    ]);
    setSettings(settingsRes);
    setVersion(versionRes);
    setForm({ base_url: settingsRes.base_url || '', bot_token: '', api_proxy: settingsRes.telegram_api_proxy || '' });
  };
  useEffect(() => { load(); }, []);
  if (!settings) return <PageLoading />;
  const save = async () => {
    setNotice('');
    setError('');
    setSaving(true);
    const body: any = { base_url: form.base_url, api_proxy: form.api_proxy };
    if (form.bot_token !== '') body.bot_token = form.bot_token;
    try {
      await api('/api/admin/settings', { method: 'PATCH', body: JSON.stringify(body) });
      setNotice('设置已保存');
      load();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  };
  const clearToken = async () => {
    if (!window.confirm('确认清空 Telegram Bot Token？清空后 Bot polling 会停止。')) return;
    setNotice('');
    setError('');
    setSaving(true);
    try {
      await api('/api/admin/settings', { method: 'PATCH', body: JSON.stringify({ bot_token: '' }) });
      setForm({ ...form, bot_token: '' });
      setNotice('Bot Token 已清空，polling 已停止');
      load();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  };
  const test = async () => {
    setNotice('');
    setError('');
    setTesting(true);
    try {
      const body: any = { api_proxy: form.api_proxy };
      if (form.bot_token !== '') body.bot_token = form.bot_token;
      const res = await api<any>('/api/admin/settings/telegram/test', { method: 'POST', body: JSON.stringify(body) });
      setNotice(`连接成功：@${res.bot.username || res.bot.first_name}`);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setTesting(false);
    }
  };
  return (
    <div className="space-y-6">
      <PageTitle title="系统设置" desc="基础配置写入 data/config.yaml，数据库类型修改后需要重启" />
      <Section title="运行信息">
        <InfoGrid rows={[
          ['配置文件', settings.config_path],
          ['数据库类型', settings.database_driver],
          ['数据库 DSN', settings.database_dsn],
          ['当前 Base URL', settings.effective_base_url],
          ['Bot Token', settings.has_bot_token ? settings.telegram_bot_token : '未配置'],
          ['Bot Polling', settings.bot_polling ? '运行中' : '未运行'],
          ['当前版本', version?.version || 'unknown'],
          ['构建提交', version?.commit || 'unknown'],
          ['构建时间', version?.buildDate || 'unknown'],
          ['运行平台', version?.platform || 'unknown']
        ]} />
      </Section>
      <Section title="可热更新配置" action={<div className="flex flex-wrap gap-2"><Button icon={<RefreshCcw />} onClick={test} disabled={testing}>{testing ? '测试中' : '测试'}</Button><Button icon={<Trash2 />} onClick={clearToken} disabled={saving}>清空 Token</Button><Button icon={<Save />} onClick={save} disabled={saving}>{saving ? '保存中' : '保存'}</Button></div>}>
        <div className="grid gap-4 md:grid-cols-2">
          <Input label="Base URL" value={form.base_url} placeholder="留空使用本机默认地址" onChange={(v) => setForm({ ...form, base_url: v })} />
          <Input label="Telegram API Proxy" value={form.api_proxy} placeholder="留空直连官方 API" onChange={(v) => setForm({ ...form, api_proxy: v })} />
          <div className="md:col-span-2"><Input label="Telegram Bot Token" value={form.bot_token} placeholder="留空表示不修改" onChange={(v) => setForm({ ...form, bot_token: v })} /></div>
        </div>
        <div className="mt-4 space-y-2">
          {notice && <Alert kind="success" text={notice} />}
          {error && <Alert kind="error" text={error} />}
        </div>
      </Section>
    </div>
  );
}

function ProfilePage({ onPasswordChanged }: { onPasswordChanged: () => void }) {
  const [form, setForm] = useState({ old_password: '', new_password: '' });
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const submit = async () => {
    setError('');
    setNotice('');
    try {
      await api('/api/auth/change-password', { method: 'POST', body: JSON.stringify(form) });
      setNotice('密码已修改，请重新登录。');
      setTimeout(() => { onPasswordChanged(); window.history.pushState({}, '', '/login'); window.dispatchEvent(new PopStateEvent('popstate')); }, 800);
    } catch (err) {
      setError((err as Error).message);
    }
  };
  return (
    <div className="space-y-6">
      <PageTitle title="管理员" desc="修改当前管理员密码" />
      <Section title="修改密码" action={<Button icon={<Save />} onClick={submit}>保存</Button>}>
        <div className="grid gap-4 md:grid-cols-2">
          <Input label="当前密码" type="password" value={form.old_password} onChange={(v) => setForm({ ...form, old_password: v })} />
          <Input label="新密码" type="password" value={form.new_password} onChange={(v) => setForm({ ...form, new_password: v })} />
        </div>
        <div className="mt-4 space-y-2">
          {notice && <Alert kind="success" text={notice} />}
          {error && <Alert kind="error" text={error} />}
        </div>
      </Section>
    </div>
  );
}

function MessageTable({ items }: { items: WebhookMessage[] }) {
  const [detail, setDetail] = useState<WebhookMessage | null>(null);
  const open = async (id: number) => {
    const res = await api<{ message: WebhookMessage }>(`/api/admin/messages/${id}`);
    setDetail(res.message);
  };
  return (
    <>
      <div className="overflow-x-auto">
        <table className="table">
          <thead><tr><th>时间</th><th>Token</th><th>标题</th><th>摘要</th><th>级别</th><th>状态</th><th>错误</th><th>来源 IP</th><th>详情</th></tr></thead>
          <tbody>
            {items.map((m) => (
              <tr key={m.id}>
                <td>{formatDate(m.created_at)}</td>
                <td>{m.token?.alias || '-'}</td>
                <td>{m.title || '-'}</td>
                <td className="max-w-xs truncate">{m.content || '-'}</td>
                <td><LevelPill value={m.level} /></td>
                <td><StatusPill value={m.delivery_status} /></td>
                <td className="max-w-xs truncate">{m.delivery_error || '-'}</td>
                <td>{m.source_ip || '-'}</td>
                <td><button className="icon-btn" title="查看详情" onClick={() => open(m.id)}><Eye className="h-4 w-4" /></button></td>
              </tr>
            ))}
            {items.length === 0 && <EmptyRow colSpan={9} text="暂无推送记录。" />}
          </tbody>
        </table>
      </div>
      {detail && <MessageModal item={detail} onClose={() => setDetail(null)} />}
    </>
  );
}

function MessageModal({ item, onClose }: { item: WebhookMessage; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4" onClick={onClose}>
      <div className="max-h-[85vh] w-full max-w-3xl overflow-auto rounded-lg bg-white p-6 shadow-soft" onClick={(e) => e.stopPropagation()}>
        <div className="mb-4 flex items-start justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold">推送详情</h2>
            <p className="text-sm text-neutral-500">{formatDate(item.created_at)}</p>
          </div>
          <button className="icon-btn" onClick={onClose} title="关闭"><XCircle className="h-4 w-4" /></button>
        </div>
        <InfoGrid rows={[
          ['标题', item.title || '-'],
          ['格式', item.format],
          ['级别', item.level],
          ['来源', item.source || '-'],
          ['状态', item.delivery_status],
          ['Telegram Message ID', String(item.telegram_message_id || '-')],
          ['来源 IP', item.source_ip || '-'],
          ['User-Agent', item.user_agent || '-'],
          ['错误', item.delivery_error || '-']
        ]} />
        <pre className="mt-4 whitespace-pre-wrap rounded-lg bg-neutral-100 p-4 text-sm text-neutral-800">{item.content || '（空内容）'}</pre>
        <pre className="mt-4 whitespace-pre-wrap rounded-lg bg-neutral-950 p-4 text-xs text-neutral-100">{item.raw_payload || '（无原始内容）'}</pre>
      </div>
    </div>
  );
}

function PageTitle({ title, desc }: { title: string; desc: string }) {
  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-normal text-neutral-950">{title}</h1>
      <p className="mt-1 text-sm text-neutral-500">{desc}</p>
    </div>
  );
}

function Section({ title, action, children }: { title: string; action?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border border-neutral-200 bg-white p-5 shadow-sm">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="font-semibold">{title}</h2>
        {action}
      </div>
      {children}
    </section>
  );
}

function Input({ label, value, onChange, type = 'text', placeholder = '' }: {
  label: string;
  value: string;
  type?: string;
  placeholder?: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm font-medium text-neutral-700">{label}</span>
      <input className="input" type={type} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} />
    </label>
  );
}

function Button({ children, icon, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { icon?: React.ReactNode }) {
  return (
    <button {...props} className={`btn ${props.className || ''}`}>
      {icon && <span className="btn-icon">{icon}</span>}
      <span>{children}</span>
    </button>
  );
}

function Alert({ kind, text }: { kind: 'success' | 'error'; text: string }) {
  return <div className={`rounded-lg px-3 py-2 text-sm ${kind === 'success' ? 'bg-emerald-50 text-emerald-800' : 'bg-rose-50 text-rose-800'}`}>{text}</div>;
}

function StatusPill({ value }: { value: string }) {
  const v = value || 'unknown';
  const styles: Record<string, string> = {
    active: 'bg-emerald-50 text-emerald-700',
    sent: 'bg-emerald-50 text-emerald-700',
    failed: 'bg-rose-50 text-rose-700',
    blocked: 'bg-rose-50 text-rose-700',
    disabled: 'bg-neutral-100 text-neutral-700'
  };
  const label: Record<string, string> = { active: '启用', sent: '成功', failed: '失败', blocked: '阻止', disabled: '禁用' };
  return <span className={`pill ${styles[v] || 'bg-neutral-100 text-neutral-700'}`}>{label[v] || v}</span>;
}

function LevelPill({ value }: { value: string }) {
  const styles: Record<string, string> = {
    info: 'bg-sky-50 text-sky-700',
    success: 'bg-emerald-50 text-emerald-700',
    warning: 'bg-amber-50 text-amber-700',
    error: 'bg-rose-50 text-rose-700'
  };
  return <span className={`pill ${styles[value] || 'bg-neutral-100 text-neutral-700'}`}>{value || 'info'}</span>;
}

function InfoGrid({ rows }: { rows: [string, string][] }) {
  return (
    <dl className="grid gap-3 md:grid-cols-2">
      {rows.map(([key, value]) => (
        <div key={key} className="rounded-lg bg-neutral-50 p-3">
          <dt className="text-xs text-neutral-500">{key}</dt>
          <dd className="mt-1 break-all text-sm font-medium text-neutral-800">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

function EmptyRow({ colSpan, text }: { colSpan: number; text: string }) {
  return <tr><td colSpan={colSpan} className="py-10 text-center text-neutral-500">{text}</td></tr>;
}

function PageLoading() {
  return <div className="py-20 text-center text-neutral-500">正在加载...</div>;
}

function formatDate(value?: string | null) {
  if (!value) return '暂无';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

createRoot(document.getElementById('root')!).render(<App />);
