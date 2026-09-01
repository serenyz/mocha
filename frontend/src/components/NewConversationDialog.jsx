import React, { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { createDirectConversation, createGroupConversation } from '../api/conversations';
import { completeMediaUpload, requestMediaUpload, uploadMediaFile } from '../api/media';
import { searchUsers } from '../api/users';

const phonePattern = /^1[3-9]\d{9}$/;
const MAX_IMAGE_SIZE = 33_554_432;

const styles = {
  overlay: {
    position: 'fixed',
    inset: 0,
    zIndex: 100,
    display: 'grid',
    placeItems: 'center',
    padding: 12,
    background: 'rgba(35, 43, 56, .46)',
    backdropFilter: 'blur(4px)',
  },
  dialog: {
    display: 'flex',
    width: 'min(680px, calc(100vw - 24px))',
    maxHeight: 'min(760px, calc(100dvh - 24px))',
    overflow: 'hidden',
    flexDirection: 'column',
    border: '1px solid #e4e8eb',
    borderRadius: 12,
    background: '#fff',
    boxShadow: '0 26px 80px rgba(29, 39, 53, .22)',
    color: '#2f3443',
  },
  header: {
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: 16,
    padding: '20px 22px 14px',
    borderBottom: '1px solid #edf0f2',
  },
  heading: { margin: 0, fontSize: 20 },
  description: { margin: '6px 0 0', color: '#7d879d', fontSize: 12, lineHeight: 1.6 },
  close: {
    width: 34,
    height: 34,
    flex: '0 0 34px',
    border: 0,
    borderRadius: 8,
    background: '#f2f4f6',
    color: '#667085',
    fontSize: 22,
    lineHeight: 1,
  },
  form: { display: 'flex', minHeight: 0, flex: 1, flexDirection: 'column', overflow: 'hidden' },
  body: { minHeight: 0, overflowY: 'auto', flex: 1, padding: '18px 22px 20px' },
  tabs: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: 4,
    marginBottom: 18,
    padding: 4,
    borderRadius: 9,
    background: '#f2f5f6',
  },
  tab: {
    minHeight: 38,
    border: 0,
    borderRadius: 7,
    background: 'transparent',
    color: '#727d90',
  },
  activeTab: { background: '#fff', color: '#3a4353', boxShadow: '0 2px 9px rgba(38, 48, 64, .08)', fontWeight: 650 },
  label: { display: 'block', marginBottom: 7, color: '#596477', fontSize: 12, fontWeight: 650 },
  input: {
    width: '100%',
    minHeight: 42,
    padding: '0 12px',
    border: '1px solid #dfe4e8',
    borderRadius: 7,
    outline: 0,
    background: '#fff',
    color: '#353d4d',
  },
  field: { marginBottom: 16 },
  avatarRow: { display: 'flex', alignItems: 'center', gap: 13 },
  avatarPreview: {
    display: 'grid',
    width: 58,
    height: 58,
    overflow: 'hidden',
    flex: '0 0 58px',
    placeItems: 'center',
    borderRadius: '50%',
    background: '#edf9f8',
    color: '#58a7a5',
    fontSize: 20,
    fontWeight: 700,
  },
  avatarImage: { width: '100%', height: '100%', objectFit: 'cover' },
  fileButton: {
    display: 'inline-flex',
    minHeight: 34,
    alignItems: 'center',
    padding: '0 12px',
    border: '1px solid #dce3e6',
    borderRadius: 7,
    background: '#fff',
    color: '#536172',
    cursor: 'pointer',
    fontSize: 11,
  },
  hint: { display: 'block', marginTop: 6, color: '#929baa', fontSize: 10, lineHeight: 1.5 },
  searchForm: { display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) auto', gap: 8, marginTop: 18 },
  searchButton: {
    minWidth: 92,
    border: 0,
    borderRadius: 7,
    background: '#68c7c5',
    color: '#fff',
    fontWeight: 650,
  },
  sectionHeading: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, margin: '18px 0 8px' },
  sectionTitle: { margin: 0, fontSize: 13 },
  selectedCount: { color: '#68a9a7', fontSize: 11 },
  selectedList: { display: 'flex', flexWrap: 'wrap', gap: 7, marginBottom: 10 },
  selectedChip: {
    display: 'inline-flex',
    maxWidth: '100%',
    minHeight: 30,
    alignItems: 'center',
    gap: 7,
    padding: '0 9px',
    border: 0,
    borderRadius: 16,
    background: '#edf9f8',
    color: '#4f8f8d',
    fontSize: 11,
  },
  resultList: { display: 'flex', maxHeight: 270, overflowY: 'auto', flexDirection: 'column', gap: 7 },
  result: {
    display: 'flex',
    width: '100%',
    minWidth: 0,
    alignItems: 'center',
    gap: 11,
    padding: 10,
    border: '1px solid #e5e9ec',
    borderRadius: 8,
    background: '#fff',
    cursor: 'pointer',
  },
  selectedResult: { borderColor: '#8fd5d3', background: '#f2fbfa' },
  userAvatar: {
    display: 'grid',
    width: 42,
    height: 42,
    overflow: 'hidden',
    flex: '0 0 42px',
    placeItems: 'center',
    borderRadius: '50%',
    background: '#e9ecf4',
    color: '#526074',
    fontWeight: 750,
  },
  userCopy: { display: 'block', minWidth: 0, flex: 1, textAlign: 'left' },
  userName: { display: 'block', overflow: 'hidden', fontSize: 12, textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  userMeta: { display: 'block', overflow: 'hidden', marginTop: 4, color: '#8b94a3', fontSize: 10, textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  check: { width: 18, height: 18, accentColor: '#68c7c5' },
  empty: { margin: 0, padding: '24px 12px', color: '#929baa', fontSize: 11, lineHeight: 1.65, textAlign: 'center' },
  loadMore: {
    minHeight: 34,
    marginTop: 8,
    border: '1px solid #dfe5e8',
    borderRadius: 7,
    background: '#fff',
    color: '#596677',
    fontSize: 11,
  },
  error: { margin: '10px 0 0', color: '#b95060', fontSize: 11, lineHeight: 1.55 },
  footer: {
    display: 'flex',
    justifyContent: 'flex-end',
    gap: 9,
    padding: '14px 22px',
    borderTop: '1px solid #edf0f2',
    background: '#fbfcfc',
  },
  secondary: { minWidth: 82, minHeight: 38, border: '1px solid #dfe4e8', borderRadius: 7, background: '#fff', color: '#5e697a' },
  primary: { minWidth: 112, minHeight: 38, border: 0, borderRadius: 7, background: '#68c7c5', color: '#fff', fontWeight: 650 },
};

function resolveAvatarURL(user) {
  const value = user?.avatar_url || '';
  if (!value) return '';
  if (/^(https?:|data:|blob:)/i.test(value)) return value;
  const base = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '');
  return `${base}${value.startsWith('/') ? value : `/${value}`}`;
}

function UserAvatar({ user }) {
  const [failed, setFailed] = useState(false);
  const src = resolveAvatarURL(user);
  useEffect(() => setFailed(false), [src]);

  return <span style={styles.userAvatar} aria-hidden="true">
    {src && !failed
      ? <img src={src} alt="" style={styles.avatarImage} onError={() => setFailed(true)} />
      : String(user?.nickname || '用').slice(0, 1).toUpperCase()}
  </span>;
}

function isAuthError(error) {
  return error?.status === 401 || ['UNAUTHENTICATED', 'INVALID_REFRESH_TOKEN', 'ACCOUNT_DISABLED'].includes(error?.code);
}

function normalizedUserId(user) {
  const id = Number(user?.id);
  return Number.isSafeInteger(id) && id > 0 ? id : null;
}

function mergeUsers(current, incoming) {
  const merged = new Map(current.map((user) => [normalizedUserId(user), user]));
  incoming.forEach((user) => {
    const id = normalizedUserId(user);
    if (id !== null) merged.set(id, user);
  });
  merged.delete(null);
  return [...merged.values()];
}

function validateAvatar(file) {
  if (!file.type?.startsWith('image/')) return '请选择图片文件作为群头像';
  if (file.size < 1 || file.size > MAX_IMAGE_SIZE) return '群头像大小必须在 32 MiB 以内';
  if (!file.name || new TextEncoder().encode(file.name).length > 255 || /[/\\]/.test(file.name)) {
    return '群头像文件名不符合要求';
  }
  return '';
}

export default function NewConversationDialog({ onClose, onCreated, onAuthExpired, initialMode = 'direct' }) {
  const dialogRef = useRef(null);
  const searchInputRef = useRef(null);
  const searchRequestRef = useRef(0);
  const uploadedAvatarRef = useRef(null);
  const closeHandlerRef = useRef(onClose);
  const submittingRef = useRef(false);
  const [mode, setMode] = useState(initialMode === 'group' ? 'group' : 'direct');
  const [query, setQuery] = useState('');
  const [results, setResults] = useState([]);
  const [lastFilters, setLastFilters] = useState(null);
  const [searchMeta, setSearchMeta] = useState({ next_cursor: null, has_more: false });
  const [searching, setSearching] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [searchError, setSearchError] = useState('');
  const [directUser, setDirectUser] = useState(null);
  const [members, setMembers] = useState([]);
  const [groupName, setGroupName] = useState('');
  const [avatarFile, setAvatarFile] = useState(null);
  const [avatarPreview, setAvatarPreview] = useState('');
  const [avatarError, setAvatarError] = useState('');
  const [submitError, setSubmitError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submitPhase, setSubmitPhase] = useState('');

  useEffect(() => { closeHandlerRef.current = onClose; }, [onClose]);
  useEffect(() => { submittingRef.current = submitting; }, [submitting]);

  useEffect(() => {
    if (!avatarFile) {
      setAvatarPreview('');
      return undefined;
    }
    const url = URL.createObjectURL(avatarFile);
    setAvatarPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [avatarFile]);

  useEffect(() => {
    const previousFocus = document.activeElement;
    const appRoot = document.getElementById('root');
    const previousRootInert = appRoot?.inert;
    const previousRootAriaHidden = appRoot?.getAttribute('aria-hidden');
    if (appRoot) {
      appRoot.inert = true;
      appRoot.setAttribute('aria-hidden', 'true');
    }
    searchInputRef.current?.focus();

    const handleKeyDown = (event) => {
      if (event.key === 'Escape' && !submittingRef.current) {
        searchRequestRef.current += 1;
        closeHandlerRef.current?.();
        return;
      }
      if (event.key === 'Tab' && dialogRef.current) {
        const focusable = [...dialogRef.current.querySelectorAll(
          'button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
        )].filter((element) => element.getClientRects().length > 0);
        if (!focusable.length) {
          event.preventDefault();
          return;
        }
        const first = focusable[0];
        const last = focusable.at(-1);
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      if (appRoot) {
        appRoot.inert = Boolean(previousRootInert);
        if (previousRootAriaHidden === null) appRoot.removeAttribute('aria-hidden');
        else appRoot.setAttribute('aria-hidden', previousRootAriaHidden);
      }
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected) previousFocus.focus();
    };
  }, []);

  const notifyAuthExpired = (error) => {
    if (!isAuthError(error) || !onAuthExpired) return false;
    onAuthExpired(error.message || '登录状态已失效，请重新登录。', error);
    return true;
  };

  const buildSearchFilters = () => {
    const value = query.trim().replace(/\s+/g, ' ');
    if (!value) throw new Error('请输入手机号或昵称');
    if (phonePattern.test(value)) return { phone: value, limit: 20 };
    if (/^\d+$/.test(value)) throw new Error('请输入正确的 11 位大陆手机号');
    if (Array.from(value).length > 50) throw new Error('昵称不能超过 50 个字符');
    return { nickname: value, limit: 20 };
  };

  const handleSearch = async (event, append = false) => {
    event?.preventDefault();
    let filters;
    try {
      filters = append ? lastFilters : buildSearchFilters();
    } catch (error) {
      setSearchError(error.message);
      return;
    }
    if (!filters || (append && (!searchMeta.has_more || searchMeta.next_cursor === null))) return;

    const requestId = searchRequestRef.current + 1;
    searchRequestRef.current = requestId;
    setSearchError('');
    setSubmitError('');
    if (append) setLoadingMore(true);
    else {
      setSearching(true);
      setResults([]);
      setLastFilters(filters);
      setSearchMeta({ next_cursor: null, has_more: false });
    }

    try {
      const response = await searchUsers(append ? { ...filters, cursor: searchMeta.next_cursor } : filters);
      if (requestId !== searchRequestRef.current) return;
      setResults((current) => append ? mergeUsers(current, response.users) : mergeUsers([], response.users));
      setSearchMeta(response.meta);
    } catch (error) {
      if (requestId !== searchRequestRef.current) return;
      if (!notifyAuthExpired(error)) setSearchError(error.message || '搜索失败，请稍后重试');
    } finally {
      if (requestId === searchRequestRef.current) {
        setSearching(false);
        setLoadingMore(false);
      }
    }
  };

  const handleQueryChange = (value) => {
    searchRequestRef.current += 1;
    setQuery(value);
    setResults([]);
    setLastFilters(null);
    setSearchMeta({ next_cursor: null, has_more: false });
    setSearching(false);
    setLoadingMore(false);
    setSearchError('');
  };

  const changeMode = (nextMode) => {
    if (submittingRef.current || nextMode === mode) return;
    setMode(nextMode);
    setSubmitError('');
    requestAnimationFrame(() => searchInputRef.current?.focus());
  };

  const toggleMember = (user) => {
    const id = normalizedUserId(user);
    if (id === null) return;
    setMembers((current) => current.some((item) => normalizedUserId(item) === id)
      ? current.filter((item) => normalizedUserId(item) !== id)
      : [...current, user]);
    setSubmitError('');
  };

  const chooseAvatar = (event) => {
    const file = event.target.files?.[0] || null;
    event.target.value = '';
    if (!file) return;
    const error = validateAvatar(file);
    if (error) {
      setAvatarError(error);
      return;
    }
    uploadedAvatarRef.current = null;
    setAvatarFile(file);
    setAvatarError('');
    setSubmitError('');
  };

  const uploadAvatarIfNeeded = async () => {
    if (!avatarFile) return null;
    if (uploadedAvatarRef.current?.file === avatarFile) return uploadedAvatarRef.current.mediaId;

    setSubmitPhase('正在上传群头像…');
    const requested = await requestMediaUpload(avatarFile);
    await uploadMediaFile(avatarFile, requested?.upload);
    const completed = await completeMediaUpload(requested?.media_id);
    if (completed?.status !== 'uploaded') throw new Error('群头像尚未完成上传');

    uploadedAvatarRef.current = { file: avatarFile, mediaId: requested.media_id };
    return requested.media_id;
  };

  const handleCreate = async (event) => {
    event.preventDefault();
    if (submittingRef.current) return;
    setSubmitError('');
    setAvatarError('');

    const selectedId = normalizedUserId(directUser);
    const normalizedName = groupName.trim().replace(/\s+/g, ' ');
    if (mode === 'direct' && selectedId === null) {
      setSubmitError('请先选择一位用户');
      return;
    }
    if (mode === 'group' && (!normalizedName || Array.from(normalizedName).length > 50)) {
      setSubmitError('群名称需为 1–50 个字符');
      return;
    }
    if (mode === 'group' && avatarFile) {
      const error = validateAvatar(avatarFile);
      if (error) {
        setAvatarError(error);
        return;
      }
    }

    submittingRef.current = true;
    setSubmitting(true);
    setSubmitPhase('正在创建会话…');
    let created;
    try {
      if (mode === 'direct') {
        created = await createDirectConversation(selectedId);
      } else {
        const avatarMediaId = await uploadAvatarIfNeeded();
        setSubmitPhase('正在创建群聊…');
        created = await createGroupConversation({
          name: normalizedName,
          avatarMediaId,
          userIds: members.map(normalizedUserId).filter((id) => id !== null),
        });
      }
    } catch (error) {
      if (!notifyAuthExpired(error)) {
        if (error?.code === 'INVALID_GROUP_NAME') setSubmitError('群名称需为 1–50 个字符');
        else setSubmitError(error.message || '创建会话失败，请稍后重试');
      }
      submittingRef.current = false;
      setSubmitting(false);
      setSubmitPhase('');
      return;
    }

    try {
      await onCreated?.(created);
    } catch (callbackError) {
      onClose?.(callbackError);
      return;
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
      setSubmitPhase('');
    }
    onClose?.();
  };

  const closeDialog = () => {
    if (submittingRef.current) return;
    searchRequestRef.current += 1;
    onClose?.();
  };

  const selectedMemberIds = new Set(members.map(normalizedUserId));
  const hasSearched = Boolean(lastFilters);
  const createLabel = submitting ? submitPhase : mode === 'direct' ? '开始私聊' : '创建群聊';

  return createPortal(
    <div
      className="new-conversation-overlay"
      style={styles.overlay}
      onMouseDown={(event) => { if (event.target === event.currentTarget) closeDialog(); }}
    >
      <section
        ref={dialogRef}
        className="new-conversation-dialog"
        style={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-labelledby="new-conversation-title"
        aria-describedby="new-conversation-description"
      >
        <header className="new-conversation-header" style={styles.header}>
          <div>
            <h2 id="new-conversation-title" style={styles.heading}>新建会话</h2>
            <p id="new-conversation-description" style={styles.description}>搜索用户开始私聊，或选择成员创建群聊。</p>
          </div>
          <button type="button" style={styles.close} onClick={closeDialog} disabled={submitting} aria-label="关闭弹窗">×</button>
        </header>

        <form style={styles.form} onSubmit={handleCreate}>
          <div className="new-conversation-body" style={styles.body}>
            <div className="new-conversation-tabs" style={styles.tabs} aria-label="会话类型">
              <button type="button" aria-pressed={mode === 'direct'} style={{ ...styles.tab, ...(mode === 'direct' ? styles.activeTab : {}) }} onClick={() => changeMode('direct')}>私聊</button>
              <button type="button" aria-pressed={mode === 'group'} style={{ ...styles.tab, ...(mode === 'group' ? styles.activeTab : {}) }} onClick={() => changeMode('group')}>群聊</button>
            </div>

            {mode === 'group' && <>
              <div style={styles.field}>
                <label htmlFor="new-group-name" style={styles.label}>群名称</label>
                <input id="new-group-name" style={styles.input} value={groupName} onChange={(event) => { setGroupName(Array.from(event.target.value).slice(0, 50).join('')); setSubmitError(''); }} placeholder="例如：产品讨论组" disabled={submitting} />
              </div>
              <div style={styles.field}>
                <span style={styles.label}>群头像（可选）</span>
                <div style={styles.avatarRow}>
                  <span style={styles.avatarPreview}>
                    {avatarPreview ? <img src={avatarPreview} alt="群头像预览" style={styles.avatarImage} /> : (groupName.trim().slice(0, 1) || '群')}
                  </span>
                  <div>
                    <label style={styles.fileButton}>
                      选择图片
                      <input type="file" accept="image/*" onChange={chooseAvatar} disabled={submitting} hidden />
                    </label>
                    {avatarFile && <><small style={styles.hint}>{avatarFile.name}</small><button type="button" style={{ ...styles.selectedChip, minHeight: 26, marginTop: 6 }} onClick={() => { uploadedAvatarRef.current = null; setAvatarFile(null); setAvatarError(''); }} disabled={submitting}>移除头像</button></>}
                    {!avatarFile && <small style={styles.hint}>支持图片，最大 32 MiB</small>}
                  </div>
                </div>
                {avatarError && <p style={styles.error} role="alert">{avatarError}</p>}
              </div>
            </>}

            <div style={styles.sectionHeading}>
              <h3 style={styles.sectionTitle}>{mode === 'direct' ? '选择聊天对象' : '选择群成员'}</h3>
              {mode === 'group' && <span style={styles.selectedCount}>已选 {members.length} 人</span>}
            </div>

            {mode === 'direct' && directUser && <div style={styles.selectedList}>
              <button type="button" style={styles.selectedChip} onClick={() => setDirectUser(null)} disabled={submitting} title="移除已选用户">
                {directUser.nickname} <span aria-hidden="true">×</span>
              </button>
            </div>}
            {mode === 'group' && members.length > 0 && <div style={styles.selectedList} aria-label="已选群成员">
              {members.map((user) => <button key={user.id} type="button" style={styles.selectedChip} onClick={() => toggleMember(user)} disabled={submitting} title={`移除 ${user.nickname}`}>
                {user.nickname} <span aria-hidden="true">×</span>
              </button>)}
            </div>}

            <div style={styles.searchForm}>
              <input
                ref={searchInputRef}
                aria-label="搜索用户"
                style={styles.input}
                value={query}
                onChange={(event) => handleQueryChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    handleSearch(event, false);
                  }
                }}
                placeholder="输入完整手机号或昵称"
                disabled={submitting}
              />
              <button type="button" style={styles.searchButton} onClick={(event) => handleSearch(event, false)} disabled={searching || submitting}>{searching ? '搜索中…' : '搜索'}</button>
            </div>
            {searchError && <p style={styles.error} role="alert">{searchError}</p>}

            <div style={{ marginTop: 12 }} aria-live="polite" aria-busy={searching}>
              {!hasSearched && !searching && <p style={styles.empty}>输入完整手机号可精确查找，也可以按昵称前缀搜索。</p>}
              {hasSearched && !searching && results.length === 0 && !searchError && <p style={styles.empty}>没有找到符合条件的用户。</p>}
              {results.length > 0 && <div style={styles.resultList}>
                {results.map((user) => {
                  const id = normalizedUserId(user);
                  const selected = mode === 'direct' ? normalizedUserId(directUser) === id : selectedMemberIds.has(id);
                  return <label key={id} style={{ ...styles.result, ...(selected ? styles.selectedResult : {}) }}>
                    <UserAvatar user={user} />
                    <span style={styles.userCopy}>
                      <strong style={styles.userName}>{user.nickname || '未命名用户'}</strong>
                      <span style={styles.userMeta}>{user.signature || [user.province, user.country].filter(Boolean).join(' · ') || '暂无公开资料'}</span>
                    </span>
                    <input
                      style={styles.check}
                      type={mode === 'direct' ? 'radio' : 'checkbox'}
                      name={mode === 'direct' ? 'direct-user' : undefined}
                      checked={selected}
                      onChange={() => mode === 'direct' ? setDirectUser(user) : toggleMember(user)}
                      disabled={submitting}
                      aria-label={`${mode === 'group' && selected ? '取消选择' : '选择'} ${user.nickname || '用户'}`}
                    />
                  </label>;
                })}
              </div>}
              {searchMeta.has_more && <button type="button" style={{ ...styles.loadMore, width: '100%' }} onClick={(event) => handleSearch(event, true)} disabled={loadingMore || submitting}>{loadingMore ? '加载中…' : '加载更多'}</button>}
            </div>

            {submitError && <p style={styles.error} role="alert">{submitError}</p>}
          </div>

          <footer className="new-conversation-footer" style={styles.footer}>
            <button type="button" style={styles.secondary} onClick={closeDialog} disabled={submitting}>取消</button>
            <button type="submit" style={styles.primary} disabled={submitting}>{createLabel}</button>
          </footer>
        </form>
      </section>
    </div>,
    document.body,
  );
}
