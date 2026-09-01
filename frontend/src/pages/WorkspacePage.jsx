import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { allCountries } from 'country-region-data';
import { logoutAccount } from '../api/auth';
import { ChatSocket } from '../api/chatSocket';
import {
  createDirectConversation,
  getConversation,
  listConversations,
  listMessages,
} from '../api/conversations';
import { completeMediaUpload, requestMediaUpload, uploadMediaFile } from '../api/media';
import { getMyProfile, searchUsers, updateMyAvatar, updateMyProfile } from '../api/users';
import NewConversationDialog from '../components/NewConversationDialog';

const phonePattern = /^1[3-9]\d{9}$/;
const genderOptions = [
  { value: 0, label: '未设置' },
  { value: 1, label: '男' },
  { value: 2, label: '女' },
];

const emptyProfileDraft = {
  nickname: '',
  gender: 0,
  signature: '',
  birthday: '',
  country: '',
  province: '',
};

const emptySearchFilters = {
  phone: '',
  nickname: '',
  country: '',
  province: '',
  age: '',
  gender: '',
};

const chinaProvinceNames = {
  AH: ['安徽', '安徽省'], BJ: ['北京', '北京市'], CQ: ['重庆', '重庆市'], FJ: ['福建', '福建省'], GS: ['甘肃', '甘肃省'], GD: ['广东', '广东省'],
  GX: ['广西', '广西壮族自治区'], GZ: ['贵州', '贵州省'], HI: ['海南', '海南省'], HE: ['河北', '河北省'], HL: ['黑龙江', '黑龙江省'], HA: ['河南', '河南省'],
  HK: ['香港', '香港特别行政区'], HB: ['湖北', '湖北省'], HN: ['湖南', '湖南省'], NM: ['内蒙古', '内蒙古自治区'], JS: ['江苏', '江苏省'], JX: ['江西', '江西省'],
  JL: ['吉林', '吉林省'], LN: ['辽宁', '辽宁省'], MO: ['澳门', '澳门特别行政区'], NX: ['宁夏', '宁夏回族自治区'], QH: ['青海', '青海省'], SN: ['陕西', '陕西省'],
  SD: ['山东', '山东省'], SH: ['上海', '上海市'], SX: ['山西', '山西省'], SC: ['四川', '四川省'], TJ: ['天津', '天津市'],
  XJ: ['新疆', '新疆维吾尔自治区'], XZ: ['西藏', '西藏自治区'], YN: ['云南', '云南省'], ZJ: ['浙江', '浙江省'],
};

const regionNames = typeof Intl.DisplayNames === 'function' ? new Intl.DisplayNames(['zh-CN'], { type: 'region' }) : null;
const countryRegionMap = new Map(allCountries.map(([name, code, regions]) => [code, { name, regions }]));
const countryOptions = allCountries
  .map(([name, code]) => ({ value: code, label: `${regionNames?.of(code) || name}（${code}）` }))
  .sort((left, right) => left.label.localeCompare(right.label, 'zh-CN'));

function getProvinceOptions(countryCode) {
  return (countryRegionMap.get(countryCode)?.regions || []).map(([name, code]) => {
    if (countryCode === 'CN' && chinaProvinceNames[code]) {
      const [value, label] = chinaProvinceNames[code];
      return { value, label };
    }
    return { value: name, label: name };
  });
}

function Icon({ name, size = 21 }) {
  const paths = {
    chats: <><path d="M5 18.5 3.5 21v-5.2A8 8 0 1 1 7 19.5Z" /><path d="M8 10h8M8 14h5" /></>,
    contacts: <><circle cx="9" cy="8" r="3" /><path d="M3.5 19a5.5 5.5 0 0 1 11 0M17 8h4M19 6v4" /></>,
    calls: <path d="M7 3H4.5A1.5 1.5 0 0 0 3 4.5C3 13.6 10.4 21 19.5 21a1.5 1.5 0 0 0 1.5-1.5V17l-4-1-1.2 3a15.7 15.7 0 0 1-10.8-10.8L8 7Z" />,
    groups: <><circle cx="8" cy="8" r="3" /><circle cx="17" cy="9" r="2.5" /><path d="M2.5 20a5.5 5.5 0 0 1 11 0M13 20a4.5 4.5 0 0 1 9 0" /></>,
    profile: <><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>,
    logout: <><path d="M10 5H5v14h5M14 8l4 4-4 4M18 12H9" /></>,
    search: <><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 4.5 4.5" /></>,
    plus: <path d="M12 5v14M5 12h14" />,
    phone: <path d="M7 3H4.5A1.5 1.5 0 0 0 3 4.5C3 13.6 10.4 21 19.5 21a1.5 1.5 0 0 0 1.5-1.5V17l-4-1-1.2 3a15.7 15.7 0 0 1-10.8-10.8L8 7Z" />,
    mail: <><rect x="3" y="5" width="18" height="14" rx="2" /><path d="m4 7 8 6 8-6" /></>,
    calendar: <><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M8 3v4M16 3v4M3 10h18" /></>,
    location: <><path d="M20 10c0 5-8 11-8 11S4 15 4 10a8 8 0 1 1 16 0Z" /><circle cx="12" cy="10" r="2.5" /></>,
    globe: <><circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18" /></>,
    clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
    cake: <><path d="M4 11h16v10H4ZM3 16c2 1.5 4 1.5 6 0 2 1.5 4 1.5 6 0 2 1.5 4 1.5 6 0" /><path d="M8 11V8M12 11V7M16 11V8" /></>,
    camera: <><path d="M4 7h3l1.5-2h7L17 7h3a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2Z" /><circle cx="12" cy="13" r="4" /></>,
    video: <><rect x="3" y="6" width="13" height="12" rx="2" /><path d="m16 10 5-3v10l-5-3" /></>,
    more: <><circle cx="12" cy="5" r="1" fill="currentColor" stroke="none" /><circle cx="12" cy="12" r="1" fill="currentColor" stroke="none" /><circle cx="12" cy="19" r="1" fill="currentColor" stroke="none" /></>,
    send: <path d="m3 11 18-8-8 18-2-8Z" />,
    mic: <><rect x="8" y="3" width="8" height="12" rx="4" /><path d="M5 11a7 7 0 0 0 14 0M12 18v3" /></>,
    attach: <path d="m20 12-8.5 8.5a6 6 0 0 1-8.5-8.5L13 2a4 4 0 0 1 5.7 5.7l-10 10a2 2 0 1 1-2.8-2.8L15 5.8" />,
    edit: <><path d="m4 20 4.3-1 10.9-10.9a2 2 0 0 0-2.8-2.8L5.5 16.2Z" /><path d="m14.8 6.8 2.8 2.8" /></>,
    check: <path d="m5 12 4 4L19 6" />,
    back: <><path d="m15 18-6-6 6-6" /><path d="M9 12h10" /></>,
    chevron: <path d="m9 18 6-6-6-6" />,
    close: <><path d="m6 6 12 12M18 6 6 18" /></>,
    retry: <><path d="M20 7v5h-5" /><path d="M19 12a7 7 0 1 0-2 5" /></>,
    file: <><path d="M6 2h8l4 4v16H6z" /><path d="M14 2v5h5" /></>,
    wifi: <><path d="M4 9a12 12 0 0 1 16 0M7 13a7 7 0 0 1 10 0M10 17a3 3 0 0 1 4 0M12 20h.01" /></>,
  };
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name] || paths.chats}</svg>;
}

function normalizeNumber(value) {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

function conversationTitle(conversation) {
  if (!conversation) return '';
  if (Number(conversation.type) === 2) return conversation.group?.name || '未命名群聊';
  return conversation.peers?.[0]?.nickname || '仅自己';
}

function conversationAvatarProfile(conversation) {
  if (!conversation) return null;
  if (Number(conversation.type) === 2) {
    return {
      nickname: conversationTitle(conversation),
      avatar_url: conversation.group?.avatar_url || '',
    };
  }
  return conversation.peers?.[0] || null;
}

function formatConversationTime(value) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const now = new Date();
  const sameDay = date.toDateString() === now.toDateString();
  if (sameDay) return new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }).format(date);
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  if (date.toDateString() === yesterday.toDateString()) return '昨天';
  if (date.getFullYear() === now.getFullYear()) return new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric' }).format(date);
  return new Intl.DateTimeFormat('zh-CN', { year: '2-digit', month: 'numeric', day: 'numeric' }).format(date);
}

function formatMessageTime(value) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date);
}

function sortConversations(items) {
  return [...items].sort((left, right) => {
    const leftTime = new Date(left.last_message?.created_at || left.created_at || 0).getTime();
    const rightTime = new Date(right.last_message?.created_at || right.created_at || 0).getTime();
    if (rightTime !== leftTime) return rightTime - leftTime;
    return normalizeNumber(right.id) - normalizeNumber(left.id);
  });
}

function mergeConversationSnapshot(current, incoming) {
  if (!current) return incoming;
  const currentSeq = normalizeNumber(current.last_message_seq);
  const incomingSeq = normalizeNumber(incoming.last_message_seq);
  const currentRead = normalizeNumber(current.last_read_seq);
  const incomingRead = normalizeNumber(incoming.last_read_seq);
  const merged = { ...current, ...incoming };

  if (currentSeq > incomingSeq) {
    merged.last_message = current.last_message;
    merged.last_message_seq = current.last_message_seq;
    merged.unread_count = current.unread_count;
  }
  if (currentRead > incomingRead) {
    merged.last_read_seq = current.last_read_seq;
    merged.unread_count = current.unread_count;
  }
  return merged;
}

function mergeMessages(current, incoming) {
  const additions = Array.isArray(incoming) ? incoming : [incoming];
  const incomingClientIds = new Set(additions.map((item) => item?.client_message_id).filter(Boolean));
  const next = current.filter((item) => !(item.local && incomingClientIds.has(item.client_message_id)));

  additions.forEach((item) => {
    if (!item) return;
    const existingIndex = next.findIndex((candidate) => (
      item.id && candidate.id === item.id
    ) || (
      item.seq && candidate.conversation_id === item.conversation_id && candidate.seq === item.seq
    ));
    const localMatch = current.find((candidate) => candidate.local && candidate.client_message_id === item.client_message_id);
    const existingMessage = existingIndex >= 0 ? next[existingIndex] : null;
    const previewAttachments = localMatch?.local_attachments || existingMessage?.attachments || existingMessage?.local_attachments || [];
    const serverAttachments = Array.isArray(item.attachments) ? item.attachments : [];
    const mergedAttachments = serverAttachments.map((attachment) => {
      const preview = previewAttachments.find((candidate) => (
        normalizeNumber(candidate.media_id) === normalizeNumber(attachment.media_id)
      ) || (
        candidate.position !== undefined && normalizeNumber(candidate.position) === normalizeNumber(attachment.position)
      ));
      const localPreviewURL = preview?.local_preview_url || preview?.preview_url || '';
      return localPreviewURL ? { ...attachment, local_preview_url: localPreviewURL } : attachment;
    });
    const merged = item.seq ? {
      ...item,
      local: false,
      status: 'sent',
      attachments: mergedAttachments,
      local_attachments: mergedAttachments.length ? undefined : localMatch?.local_attachments || item.local_attachments,
    } : item;
    if (existingIndex >= 0) next[existingIndex] = { ...next[existingIndex], ...merged };
    else next.push(merged);
  });

  return next.sort((left, right) => {
    if (left.seq && right.seq) return left.seq - right.seq;
    if (left.seq) return -1;
    if (right.seq) return 1;
    return normalizeNumber(left.local_order) - normalizeNumber(right.local_order);
  });
}

function createClientMessageId() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`;
}

function resolveAssetURL(value) {
  if (!value) return '';
  if (/^(https?:|data:|blob:)/i.test(value)) return value;
  const base = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '');
  return `${base}${value.startsWith('/') ? value : `/${value}`}`;
}

function resolveAvatarURL(profile) {
  return resolveAssetURL(profile?.avatar_url || profile?.avata_url || '');
}

function attachmentName(attachment, index = 0) {
  return attachment?.filename || attachment?.name || attachment?.file?.name || `附件 ${index + 1}`;
}

function attachmentMIMEType(attachment) {
  return attachment?.mime_type || attachment?.file?.type || '';
}

function isImageAttachment(attachment) {
  return attachment?.type === 'image' || attachment?.media?.type === 'image' || attachmentMIMEType(attachment).startsWith('image/');
}

function formatAttachmentSize(value) {
  const size = normalizeNumber(value);
  if (!size) return '';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${Math.ceil(size / 102.4) / 10} KiB`;
  return `${Math.ceil(size / (1024 * 102.4)) / 10} MiB`;
}

function UserAvatar({ profile, size = 'md', className = '' }) {
  const [failed, setFailed] = useState(false);
  const src = resolveAvatarURL(profile);
  useEffect(() => setFailed(false), [src]);
  return <span className={`native-avatar avatar-${size} ${className}`}>{src && !failed ? <img src={src} alt={profile?.nickname || '用户头像'} onError={() => setFailed(true)} /> : <b>{profile?.nickname?.slice(0, 1)?.toUpperCase() || 'M'}</b>}</span>;
}

function DemoAvatar({ item, size = 'md' }) {
  return <span className={`native-avatar avatar-${size} demo-avatar ${item.color || 'blue'}`}><b>{item.initial}</b>{item.online && <i />}</span>;
}

function Spinner() {
  return <span className="native-spinner" aria-hidden="true" />;
}

function MessageAttachmentPreview({ attachment, index, message, onPreviewError, onRemoteLoad }) {
  const [primaryFailed, setPrimaryFailed] = useState(false);
  const [fallbackFailed, setFallbackFailed] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const name = attachmentName(attachment, index);
  const primarySrc = resolveAssetURL(attachment?.url || attachment?.media?.url || '');
  const fallbackSrc = resolveAssetURL(attachment?.local_preview_url || attachment?.preview_url || '');
  const useFallback = !primarySrc || primaryFailed;
  const src = useFallback ? fallbackSrc : primarySrc;
  const image = isImageAttachment(attachment);
  const size = formatAttachmentSize(attachment?.size || attachment?.file?.size);

  useEffect(() => {
    setPrimaryFailed(false);
    setFallbackFailed(false);
    setRefreshing(false);
  }, [primarySrc, fallbackSrc]);

  const refreshPreview = () => {
    if (!message?.seq || !onPreviewError) return;
    setRefreshing(true);
    Promise.resolve(onPreviewError(message)).finally(() => setRefreshing(false));
  };

  if (image && src && !fallbackFailed) {
    return <a className="message-image-attachment" href={src} target="_blank" rel="noreferrer" aria-label={`查看图片 ${name}`}>
      <img
        src={src}
        alt={name}
        loading="lazy"
        onLoad={() => {
          if (!useFallback && fallbackSrc && !message?.local && message?.seq) onRemoteLoad?.(fallbackSrc);
        }}
        onError={() => {
          if (!useFallback && fallbackSrc) {
            setPrimaryFailed(true);
            refreshPreview();
          } else if (!useFallback) {
            setPrimaryFailed(true);
            refreshPreview();
          } else {
            setFallbackFailed(true);
          }
        }}
      />
      <span className="message-image-caption"><span>{name}</span>{size && <small>{size}</small>}</span>
    </a>;
  }

  const unavailable = image && (primaryFailed || fallbackFailed);
  const content = <><Icon name={image ? 'camera' : 'file'} size={15} /><span>{name}</span><small>{unavailable ? refreshing ? '预览失效，正在刷新…' : '图片暂时无法预览' : size || (attachment?.media_id ? `#${attachment.media_id}` : '')}</small></>;
  return attachment?.url && !image
    ? <a className="message-attachment" href={resolveAssetURL(attachment.url)} target="_blank" rel="noreferrer">{content}</a>
    : <span className={`message-attachment${unavailable ? ' preview-failed' : ''}`}>{content}</span>;
}

function ComposerAttachmentPreview({ attachment, onRemove }) {
  const [previewFailed, setPreviewFailed] = useState(false);
  const name = attachmentName(attachment);
  const previewSrc = resolveAssetURL(attachment?.preview_url || attachment?.url || attachment?.media?.url || '');
  const image = isImageAttachment(attachment);

  useEffect(() => setPreviewFailed(false), [previewSrc]);

  return <div className={`composer-attachment ${attachment.status}${image ? ' image' : ''}`}>
    {image && previewSrc && !previewFailed
      ? <img className="composer-attachment-preview" src={previewSrc} alt={name} onError={() => setPreviewFailed(true)} />
      : <span className="composer-attachment-icon"><Icon name={image ? 'camera' : 'file'} size={16} /></span>}
    <span className="composer-attachment-info"><b>{name}</b><small>{attachment.status === 'uploading' ? '上传中…' : attachment.status === 'failed' ? attachment.error : '已就绪'}</small></span>
    <button type="button" onClick={() => onRemove(attachment.key)} aria-label={`移除 ${name}`}><Icon name="close" size={13} /></button>
  </div>;
}

function Notice({ notice }) {
  if (!notice) return null;
  return <div className={`native-notice ${notice.status}`} role="alert"><strong>{notice.title}</strong><span>{notice.description}</span></div>;
}

function genderLabel(value) {
  return genderOptions.find((item) => item.value === Number(value))?.label || '未设置';
}

function todayValue() {
  const now = new Date();
  return new Date(now.getTime() - now.getTimezoneOffset() * 60000).toISOString().slice(0, 10);
}

function isCalendarDate(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const [year, month, day] = value.split('-').map(Number);
  const date = new Date(year, month - 1, day);
  return date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day;
}

function getAge(birthday) {
  if (!isCalendarDate(birthday)) return null;
  const [year, month, day] = birthday.split('-').map(Number);
  const today = new Date();
  let age = today.getFullYear() - year;
  if (today.getMonth() + 1 < month || (today.getMonth() + 1 === month && today.getDate() < day)) age -= 1;
  return age >= 0 ? age : null;
}

function countryLabel(value) {
  const country = String(value || '').trim();
  if (!country) return '';

  const code = country.toUpperCase();
  if (!/^[A-Z]{2}$/.test(code)) return country;
  return regionNames?.of(code) || countryRegionMap.get(code)?.name || country;
}

function regionLabel(profile) {
  return [profile?.province, countryLabel(profile?.country)].filter(Boolean).join(' · ') || '未设置';
}

function joinedDateLabel(value) {
  if (!value) return '未知';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '未知';
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' }).format(date);
}

function ProfileInfoList({ items }) {
  return <ul className="profile-info-list">{items.map((item) => <li key={item.label} aria-label={`${item.label}：${item.value}`}>
    <span className="profile-info-icon" title={item.label}><Icon name={item.icon} size={16} /></span>
    <span className="profile-info-value">{item.value}</span>
  </li>)}</ul>;
}

function ProfileEditField({ label, icon, error, children, className = '' }) {
  return <label className={`profile-edit-field ${className}`}>
    <span className="profile-edit-icon" title={label}><Icon name={icon} size={16} /></span>
    <span className="profile-edit-control"><span className="sr-only">{label}</span>{children}{error && <small>{error}</small>}</span>
  </label>;
}

function SearchField({ label, icon, error, children, className = '' }) {
  return <label className={`contact-icon-field ${className}`}>
    <span className="contact-field-icon" title={label}><Icon name={icon} size={15} /></span>
    <span className="contact-field-control"><span className="sr-only">{label}</span>{children}{error && <small>{error}</small>}</span>
  </label>;
}

function SearchResult({ user, onOpen }) {
  const age = getAge(user.birthday);
  return <button className="contact-result" type="button" onClick={() => onOpen(user)} aria-label={`查看 ${user.nickname} 的资料`}>
    <UserAvatar profile={user} size="lg" />
    <span className="contact-result-copy"><strong>{user.nickname}</strong><span className="contact-result-signature">{user.signature || '暂无个性签名'}</span><span className="contact-result-meta"><span>{genderLabel(user.gender)}</span>{age !== null && <span>{age} 岁</span>}<span>{regionLabel(user)}</span></span></span>
    <span className="contact-result-arrow"><Icon name="chevron" size={17} /></span>
  </button>;
}

function ContactDetail({ user, onBack, onStartChat, startingChat }) {
  const age = getAge(user.birthday);
  const detailItems = [
    { label: '性别', icon: 'profile', value: genderLabel(user.gender) },
    { label: '年龄', icon: 'cake', value: age === null ? '未设置' : `${age} 岁` },
    { label: '生日', icon: 'calendar', value: user.birthday || '未设置' },
    { label: '地区', icon: 'location', value: regionLabel(user) },
  ];
  return <>
    <header className="context-head contact-detail-head"><button className="icon-button" type="button" onClick={onBack} aria-label="返回搜索结果"><Icon name="back" size={18} /></button><h1>用户详情</h1><span /></header>
    <div className="contact-detail-cover" />
    <div className="contact-detail-content">
      <div className="reference-avatar-wrap"><UserAvatar profile={user} size="xl" /></div>
      <h2>{user.nickname}</h2><span>公开资料</span>
      <p>{user.signature || '这个人还没有填写个性签名。'}</p>
      <ProfileInfoList items={detailItems} />
      <button className="contact-start-chat" type="button" onClick={() => onStartChat(user)} disabled={startingChat}>
        {startingChat ? <><Spinner />正在创建会话</> : <><Icon name="chats" size={17} />发起私聊</>}
      </button>
    </div>
  </>;
}

function connectionCopy(status) {
  const copies = {
    online: '实时在线',
    connecting: '正在连接',
    reconnecting: '正在重连',
    replaced: '已在其他窗口连接',
    offline: '离线',
  };
  return copies[status] || '离线';
}

function messageDeliveryLabel(message, progress, currentUserId) {
  if (message.local) {
    if (message.status === 'failed') return message.error_message || '发送失败';
    if (message.status === 'unknown') return '状态未知';
    if (message.status === 'accepted') return '发送中';
    return '待确认';
  }
  if (!message.seq) return '';
  const values = Object.entries(progress || {})
    .filter(([userId]) => normalizeNumber(userId) !== normalizeNumber(currentUserId))
    .map(([, value]) => value);
  if (values.some((item) => normalizeNumber(item.read) >= message.seq)) return '已读';
  if (values.some((item) => normalizeNumber(item.delivered) >= message.seq)) return '已送达';
  return '已发送';
}

function ConversationArea({
  conversation,
  currentUser,
  state,
  connection,
  progress,
  composerText,
  attachments,
  onComposerText,
  onFiles,
  onRemoveAttachment,
  onSend,
  onRetry,
  onLoadOlder,
  onRefreshAttachment,
  onReleasePreviewURL,
  onReadVisibilityChange,
  onBack,
  onReconnect,
}) {
  const listRef = useRef(null);
  const fileRef = useRef(null);
  const previousConversationRef = useRef(null);
  const atBottomRef = useRef(true);
  const messages = state?.items || [];
  const latestKey = messages.at(-1)?.id || messages.at(-1)?.client_message_id || '';

  useEffect(() => {
    const list = listRef.current;
    if (!list || !conversation) return;
    const changedConversation = previousConversationRef.current !== conversation.id;
    const nearBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 120;
    if (changedConversation || atBottomRef.current || nearBottom) {
      window.requestAnimationFrame(() => {
        list.scrollTop = list.scrollHeight;
        atBottomRef.current = true;
        onReadVisibilityChange(true);
      });
    } else {
      atBottomRef.current = nearBottom;
      onReadVisibilityChange(nearBottom);
    }
    previousConversationRef.current = conversation.id;
  }, [conversation?.id, latestKey]);

  const loadOlder = async () => {
    const list = listRef.current;
    const previousHeight = list?.scrollHeight || 0;
    await onLoadOlder();
    window.requestAnimationFrame(() => {
      if (list) list.scrollTop += list.scrollHeight - previousHeight;
    });
  };

  if (!conversation) {
    return <section className="reference-chat chat-welcome">
      <div className="panel-empty"><span className="welcome-mark"><Icon name="chats" size={42} /></span><h2>选择一段会话</h2><p>消息会在这里实时同步。断线重连后，客户端会按消息序号自动补齐缺口。</p></div>
    </section>;
  }

  const canSend = connection.status === 'online'
    && !attachments.some((item) => item.status === 'uploading')
    && (composerText.trim() || attachments.some((item) => item.status === 'ready'));
  const senderProfile = (senderId) => {
    if (normalizeNumber(senderId) === normalizeNumber(currentUser?.id)) return currentUser;
    return conversation.peers?.find((item) => normalizeNumber(item.id) === normalizeNumber(senderId)) || { nickname: '会话成员' };
  };

  return <section className="reference-chat">
    <header className="reference-chat-head">
      <button className="mobile-chat-back" type="button" onClick={onBack} aria-label="返回会话列表"><Icon name="back" size={20} /></button>
      <div className="chat-person"><UserAvatar profile={conversationAvatarProfile(conversation)} /><span><strong>{conversationTitle(conversation)}</strong><small>{Number(conversation.type) === 2 ? `${conversation.member_count || 1} 位成员` : '私聊会话'}</small></span></div>
      <button className={`connection-pill ${connection.status}`} type="button" onClick={connection.status === 'online' ? undefined : onReconnect} title={connection.status === 'online' ? 'WebSocket 已认证' : '点击重新连接'}>
        {connection.status === 'connecting' || connection.status === 'reconnecting' ? <Spinner /> : <Icon name="wifi" size={15} />}
        <span>{connectionCopy(connection.status)}</span>
      </button>
    </header>
    <div className="reference-message-list" ref={listRef} aria-live="polite" onScroll={(event) => {
      const element = event.currentTarget;
      const isAtBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 80;
      atBottomRef.current = isAtBottom;
      onReadVisibilityChange(isAtBottom);
    }}>
      {state?.hasOlder && <button className="load-older-messages" type="button" onClick={loadOlder} disabled={state.loadingOlder}>{state.loadingOlder ? <><Spinner />加载中</> : '查看更早消息'}</button>}
      {state?.loading && messages.length === 0 && <div className="message-panel-state"><Spinner /><span>正在载入消息…</span></div>}
      {state?.error && messages.length === 0 && <div className="message-panel-state danger"><span>{state.error}</span></div>}
      {!state?.loading && !state?.error && messages.length === 0 && <div className="message-panel-state"><Icon name="chats" size={30} /><span>还没有消息，打个招呼吧。</span></div>}
      {messages.map((message) => {
        const outgoing = normalizeNumber(message.sender_id) === normalizeNumber(currentUser?.id);
        const sender = senderProfile(message.sender_id);
        const localAttachments = message.local_attachments || [];
        const serverAttachments = message.attachments || [];
        const attachmentItems = serverAttachments.length ? serverAttachments : localAttachments;
        return <div className={`message-row ${outgoing ? 'outgoing' : 'incoming'}${message.status === 'failed' ? ' failed' : ''}`} key={message.id || message.client_message_id}>
          {!outgoing && <UserAvatar profile={sender} size="sm" />}
          <div className="message-content">
            {!outgoing && Number(conversation.type) === 2 && <strong>{sender.nickname}</strong>}
            {message.text && <p>{message.text}</p>}
            {attachmentItems.length > 0 && <div className="message-attachments">{attachmentItems.map((attachment, index) => <MessageAttachmentPreview
              attachment={attachment}
              index={index}
              key={attachment.id || attachment.media_id || attachment.key || index}
              message={message}
              onPreviewError={onRefreshAttachment}
              onRemoteLoad={onReleasePreviewURL}
            />)}</div>}
            <time>{formatMessageTime(message.created_at || message.local_created_at)}{outgoing && <> · {messageDeliveryLabel(message, progress, currentUser?.id)}</>}</time>
            {outgoing && (message.status === 'failed' || message.status === 'unknown') && <button className="message-retry" type="button" onClick={() => onRetry(message)}><Icon name="retry" size={13} />重试</button>}
          </div>
        </div>;
      })}
    </div>
    {attachments.length > 0 && <div className="composer-attachments">{attachments.map((attachment) => <ComposerAttachmentPreview attachment={attachment} key={attachment.key} onRemove={onRemoveAttachment} />)}</div>}
    <form className="reference-composer" onSubmit={onSend}>
      <input ref={fileRef} className="composer-file-input" type="file" multiple hidden onChange={(event) => { onFiles(event.target.files); event.target.value = ''; }} />
      <button type="button" onClick={() => fileRef.current?.click()} aria-label="添加附件" title="添加附件"><Icon name="attach" /></button>
      <textarea value={composerText} onChange={(event) => onComposerText(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); if (canSend) onSend(event); } }} placeholder={connection.status === 'online' ? '输入消息，Enter 发送…' : '实时连接恢复后即可发送'} aria-label="输入消息" maxLength={4000} rows={1} />
      <button className="send-button" type="submit" aria-label="发送消息" disabled={!canSend}><Icon name="send" /></button>
    </form>
  </section>;
}

export default function WorkspacePage({ session, onSignedOut }) {
  const contextRef = useRef(null);
  const avatarInputRef = useRef(null);
  const socketRef = useRef(null);
  const socketEventRef = useRef(null);
  const connectionRef = useRef({ status: 'connecting' });
  const signedOutRef = useRef(onSignedOut);
  const selectedConversationRef = useRef(null);
  const viewRef = useRef('chats');
  const profileRef = useRef(null);
  const conversationItemsRef = useRef([]);
  const sequenceRef = useRef(new Map());
  const pendingSequenceRef = useRef(new Map());
  const syncingRef = useRef(new Map());
  const syncRetryTimersRef = useRef(new Map());
  const syncRetryCountsRef = useRef(new Map());
  const reportedDeliveredRef = useRef(new Map());
  const reportedReadRef = useRef(new Map());
  const messageTimersRef = useRef(new Map());
  const searchRequestRef = useRef(0);
  const conversationRequestRef = useRef(0);
  const hadOnlineConnectionRef = useRef(false);
  const chatAtBottomRef = useRef(false);
  const sendGuardRef = useRef(false);
  const composerAttachmentsRef = useRef([]);
  const previewURLsRef = useRef(new Set());
  const localMessagePreviewURLsRef = useRef(new Map());
  const previewReleaseTimersRef = useRef(new Map());
  const mediaRefreshesRef = useRef(new Map());
  const [view, setView] = useState('chats');
  const [profile, setProfile] = useState(null);
  const [draft, setDraft] = useState(emptyProfileDraft);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [avatarUploading, setAvatarUploading] = useState(false);
  const [editingProfile, setEditingProfile] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);
  const [notice, setNotice] = useState(null);
  const [errors, setErrors] = useState({});
  const [searchMode, setSearchMode] = useState('profile');
  const [searchFilters, setSearchFilters] = useState(emptySearchFilters);
  const [searchErrors, setSearchErrors] = useState({});
  const [searchResults, setSearchResults] = useState([]);
  const [searchMeta, setSearchMeta] = useState({ next_cursor: null, has_more: false });
  const [lastSearch, setLastSearch] = useState(null);
  const [selectedUser, setSelectedUser] = useState(null);
  const [searching, setSearching] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [conversationItems, setConversationItems] = useState([]);
  const [conversationMeta, setConversationMeta] = useState({ next_cursor: null, has_more: false });
  const [conversationLoading, setConversationLoading] = useState(true);
  const [conversationLoadingMore, setConversationLoadingMore] = useState(false);
  const [conversationNotice, setConversationNotice] = useState(null);
  const [conversationQuery, setConversationQuery] = useState('');
  const [selectedConversationId, setSelectedConversationId] = useState(null);
  const [messageStates, setMessageStates] = useState({});
  const [connection, setConnection] = useState({ status: 'connecting' });
  const [progressByConversation, setProgressByConversation] = useState({});
  const [composerText, setComposerText] = useState('');
  const [composerAttachments, setComposerAttachments] = useState([]);
  const [showConversationDialog, setShowConversationDialog] = useState(false);
  const [conversationDialogMode, setConversationDialogMode] = useState('direct');
  const [startingChatUserId, setStartingChatUserId] = useState(null);
  const provinceOptions = useMemo(() => {
    const options = getProvinceOptions(draft.country);
    if (draft.province && !options.some((item) => item.value === draft.province)) return [{ value: draft.province, label: draft.province }, ...options];
    return options;
  }, [draft.country, draft.province]);
  const searchProvinceOptions = useMemo(() => getProvinceOptions(searchFilters.country), [searchFilters.country]);
  const selectedConversation = useMemo(
    () => conversationItems.find((item) => normalizeNumber(item.id) === normalizeNumber(selectedConversationId)) || null,
    [conversationItems, selectedConversationId],
  );
  const visibleConversations = useMemo(() => {
    const query = conversationQuery.trim().toLocaleLowerCase('zh-CN');
    const items = view === 'groups' ? conversationItems.filter((item) => Number(item.type) === 2) : conversationItems;
    if (!query) return items;
    return items.filter((item) => {
      const haystack = [conversationTitle(item), item.last_message?.text, ...(item.peers || []).map((peer) => peer.nickname)].filter(Boolean).join(' ').toLocaleLowerCase('zh-CN');
      return haystack.includes(query);
    });
  }, [conversationItems, conversationQuery, view]);

  useEffect(() => { signedOutRef.current = onSignedOut; }, [onSignedOut]);
  useEffect(() => { selectedConversationRef.current = selectedConversationId; }, [selectedConversationId]);
  useEffect(() => { viewRef.current = view; }, [view]);
  useEffect(() => { profileRef.current = profile; }, [profile]);
  useEffect(() => { conversationItemsRef.current = conversationItems; }, [conversationItems]);
  useEffect(() => () => {
    previewReleaseTimersRef.current.forEach((timer) => window.clearTimeout(timer));
    previewReleaseTimersRef.current.clear();
    previewURLsRef.current.forEach((url) => URL.revokeObjectURL(url));
    previewURLsRef.current.clear();
    localMessagePreviewURLsRef.current.clear();
    mediaRefreshesRef.current.clear();
  }, []);
  useEffect(() => {
    if (!localMessagePreviewURLsRef.current.size) return;
    Object.values(messageStates).forEach((state) => {
      (state.items || []).forEach((message) => {
        if (!message.local && message.seq && localMessagePreviewURLsRef.current.has(message.client_message_id)) {
          settleLocalMessagePreviewURLs(message);
        }
      });
    });
  }, [messageStates]);

  function updateComposerAttachments(updater) {
    const next = updater(composerAttachmentsRef.current);
    composerAttachmentsRef.current = next;
    setComposerAttachments(next);
  }

  function registerPreviewURL(file) {
    if (!file?.type?.startsWith('image/') || typeof URL.createObjectURL !== 'function') return '';
    const url = URL.createObjectURL(file);
    previewURLsRef.current.add(url);
    return url;
  }

  function releasePreviewURL(url) {
    if (!url || !previewURLsRef.current.has(url)) return;
    const timer = previewReleaseTimersRef.current.get(url);
    if (timer) window.clearTimeout(timer);
    previewReleaseTimersRef.current.delete(url);
    URL.revokeObjectURL(url);
    previewURLsRef.current.delete(url);
  }

  function schedulePreviewURLRelease(url) {
    if (!url || !previewURLsRef.current.has(url) || previewReleaseTimersRef.current.has(url)) return;
    const timer = window.setTimeout(() => releasePreviewURL(url), 60_000);
    previewReleaseTimersRef.current.set(url, timer);
  }

  function settleLocalMessagePreviewURLs(message) {
    const urls = localMessagePreviewURLsRef.current.get(message?.client_message_id) || [];
    if (!urls.length) return;
    localMessagePreviewURLsRef.current.delete(message.client_message_id);
    const conversationId = normalizeNumber(message.conversation_id);
    const mounted = normalizeNumber(selectedConversationRef.current) === conversationId
      && (viewRef.current === 'chats' || viewRef.current === 'groups');
    urls.forEach((url) => {
      if (mounted) schedulePreviewURLRelease(url);
      else releasePreviewURL(url);
    });
  }

  function clearComposerAttachments() {
    composerAttachmentsRef.current.forEach((attachment) => releasePreviewURL(attachment.preview_url));
    composerAttachmentsRef.current = [];
    setComposerAttachments([]);
  }

  function handleAuthenticationError(error) {
    if (error?.status === 401 || error?.code === 'INVALID_REFRESH_TOKEN' || error?.code === 'ACCOUNT_DISABLED') {
      signedOutRef.current?.(error.message || '登录状态已失效，请重新登录。');
      return true;
    }
    return false;
  }

  function isConversationVisible(conversationId) {
    return normalizeNumber(selectedConversationRef.current) === normalizeNumber(conversationId)
      && (viewRef.current === 'chats' || viewRef.current === 'groups')
      && document.visibilityState === 'visible'
      && chatAtBottomRef.current;
  }

  function updateMessageState(conversationId, updater) {
    const key = String(conversationId);
    setMessageStates((current) => {
      const previous = current[key] || {
        items: [],
        loading: false,
        loadingOlder: false,
        initialized: false,
        hasOlder: false,
        nextBeforeSeq: null,
        error: null,
      };
      return { ...current, [key]: updater(previous) };
    });
  }

  function mergeConversationMessage(conversationId, incoming) {
    updateMessageState(conversationId, (current) => ({
      ...current,
      items: mergeMessages(current.items, incoming),
      error: null,
    }));
  }

  function replaceConversation(conversation) {
    if (!conversation?.id) return;
    setConversationItems((current) => {
      const index = current.findIndex((item) => normalizeNumber(item.id) === normalizeNumber(conversation.id));
      if (index < 0) {
        const added = sortConversations([conversation, ...current]);
        conversationItemsRef.current = added;
        return added;
      }
      const next = [...current];
      next[index] = mergeConversationSnapshot(next[index], conversation);
      const sorted = sortConversations(next);
      conversationItemsRef.current = sorted;
      return sorted;
    });
  }

  function updateConversationFromMessage(message, active = false) {
    const found = conversationItemsRef.current.some((item) => normalizeNumber(item.id) === normalizeNumber(message.conversation_id));
    setConversationItems((current) => {
      const next = current.map((conversation) => {
        if (normalizeNumber(conversation.id) !== normalizeNumber(message.conversation_id)) return conversation;
        const fromOther = normalizeNumber(message.sender_id) !== normalizeNumber(profileRef.current?.id);
        const previousSeq = normalizeNumber(conversation.last_message_seq);
        const isNew = normalizeNumber(message.seq) > previousSeq;
        return {
          ...conversation,
          last_message: isNew || !conversation.last_message ? {
            id: message.id,
            seq: message.seq,
            sender_id: message.sender_id,
            type: message.type,
            text: message.text,
            created_at: message.created_at,
          } : conversation.last_message,
          last_message_seq: Math.max(previousSeq, normalizeNumber(message.seq)),
          unread_count: active ? 0 : fromOther && isNew ? normalizeNumber(conversation.unread_count) + 1 : normalizeNumber(conversation.unread_count),
        };
      });
      const sorted = sortConversations(next);
      conversationItemsRef.current = sorted;
      return sorted;
    });
    if (!found) refreshConversation(message.conversation_id);
  }

  function reportProgress(type, conversationId, seq) {
    const normalizedSeq = normalizeNumber(seq);
    if (!conversationId || normalizedSeq <= 0 || connectionRef.current.status !== 'online') return false;
    const conversation = conversationItemsRef.current.find((item) => normalizeNumber(item.id) === normalizeNumber(conversationId));
    if (conversation && normalizedSeq <= normalizeNumber(conversation.joined_seq)) return false;
    const reported = type === 'conversation.read' ? reportedReadRef.current : reportedDeliveredRef.current;
    if (normalizeNumber(reported.get(conversationId)) >= normalizedSeq) return true;
    const sent = socketRef.current?.send(type, { conversation_id: normalizeNumber(conversationId), seq: normalizedSeq });
    if (sent) {
      reported.set(conversationId, normalizedSeq);
      if (type === 'conversation.read') {
        setConversationItems((current) => current.map((item) => normalizeNumber(item.id) === normalizeNumber(conversationId) ? {
          ...item,
          last_read_seq: Math.max(normalizeNumber(item.last_read_seq), normalizedSeq),
          unread_count: 0,
        } : item));
      }
    }
    return Boolean(sent);
  }

  async function refreshConversation(conversationId) {
    try {
      const conversation = await getConversation(conversationId);
      replaceConversation(conversation);
      const id = normalizeNumber(conversation.id);
      if (!sequenceRef.current.has(id)) sequenceRef.current.set(id, Math.max(normalizeNumber(conversation.joined_seq), normalizeNumber(conversation.last_read_seq)));
      const pendingSeq = normalizeNumber(pendingSequenceRef.current.get(id));
      if (pendingSeq > normalizeNumber(sequenceRef.current.get(id))) {
        syncConversationAfter(id, sequenceRef.current.get(id));
      }
      return conversation;
    } catch (error) {
      if (!handleAuthenticationError(error) && error?.code !== 'CONVERSATION_NOT_FOUND') {
        setConversationNotice({ status: 'danger', title: '会话刷新失败', description: error.message });
      }
      return null;
    }
  }

  async function syncConversationAfter(conversationId, afterSeq) {
    const id = normalizeNumber(conversationId);
    if (!id) return null;
    window.clearTimeout(syncRetryTimersRef.current.get(id));
    syncRetryTimersRef.current.delete(id);
    if (syncingRef.current.has(id)) return syncingRef.current.get(id);

    const task = (async () => {
      let cursor = Math.max(normalizeNumber(afterSeq), normalizeNumber(sequenceRef.current.get(id)));
      let latestMessage = null;
      let completed = false;
      try {
        for (let page = 0; page < 400; page += 1) {
          const target = normalizeNumber(pendingSequenceRef.current.get(id));
          if (page > 0 && target <= cursor) break;
          const payload = await listMessages(id, { afterSeq: cursor, limit: 100 });
          const items = Array.isArray(payload.items) ? payload.items : [];
          if (items.length) {
            mergeConversationMessage(id, items);
            latestMessage = items.at(-1);
            const maxSeq = Math.max(...items.map((item) => normalizeNumber(item.seq)));
            if (maxSeq <= cursor) break;
            cursor = maxSeq;
          }
          const latestTarget = normalizeNumber(pendingSequenceRef.current.get(id));
          if (cursor >= latestTarget) break;
          if (!payload.has_more && items.length === 0) break;
          if (payload.has_more && payload.next_after_seq !== null && payload.next_after_seq !== undefined) {
            const nextCursor = normalizeNumber(payload.next_after_seq);
            if (nextCursor > cursor) cursor = nextCursor;
          }
        }
        const finalCursor = Math.max(normalizeNumber(sequenceRef.current.get(id)), cursor);
        sequenceRef.current.set(id, finalCursor);
        if (normalizeNumber(pendingSequenceRef.current.get(id)) <= finalCursor) {
          pendingSequenceRef.current.delete(id);
          syncRetryCountsRef.current.delete(id);
        }
        if (latestMessage) updateConversationFromMessage(latestMessage, isConversationVisible(id));
        reportProgress('conversation.delivered', id, finalCursor);
        if (isConversationVisible(id)) reportProgress('conversation.read', id, finalCursor);
        completed = true;
        return finalCursor;
      } catch (error) {
        if (error?.code === 'CONVERSATION_NOT_FOUND') pendingSequenceRef.current.delete(id);
        if (!handleAuthenticationError(error)) {
          updateMessageState(id, (current) => ({ ...current, error: error.message || '消息同步失败' }));
        }
        return null;
      } finally {
        syncingRef.current.delete(id);
        const remainingTarget = normalizeNumber(pendingSequenceRef.current.get(id));
        const currentSeq = normalizeNumber(sequenceRef.current.get(id));
        if (completed && remainingTarget > currentSeq) {
          const retries = normalizeNumber(syncRetryCountsRef.current.get(id));
          if (retries < 8) {
            syncRetryCountsRef.current.set(id, retries + 1);
            const timer = window.setTimeout(() => {
              syncRetryTimersRef.current.delete(id);
              syncConversationAfter(id, sequenceRef.current.get(id));
            }, Math.min(4000, 300 * (2 ** retries)));
            syncRetryTimersRef.current.set(id, timer);
          }
        }
      }
    })();
    syncingRef.current.set(id, task);
    return task;
  }

  async function loadConversationPage({ append = false, resync = false } = {}) {
    const requestId = ++conversationRequestRef.current;
    if (append) setConversationLoadingMore(true);
    else {
      setConversationLoading(true);
      setConversationNotice(null);
    }
    try {
      const payload = await listConversations({
        cursor: append ? conversationMeta.next_cursor : undefined,
        limit: 50,
      });
      if (requestId !== conversationRequestRef.current) return;
      const items = Array.isArray(payload.items) ? payload.items : [];
      setConversationItems((current) => {
        const merged = [...current];
        items.forEach((conversation) => {
          const index = merged.findIndex((item) => normalizeNumber(item.id) === normalizeNumber(conversation.id));
          if (index >= 0) merged[index] = mergeConversationSnapshot(merged[index], conversation);
          else merged.push(conversation);
        });
        const sorted = sortConversations(merged);
        conversationItemsRef.current = sorted;
        return sorted;
      });
      setConversationMeta({ next_cursor: payload.next_cursor ?? null, has_more: Boolean(payload.has_more) });

      items.forEach((conversation) => {
        const id = normalizeNumber(conversation.id);
        const serverSeq = normalizeNumber(conversation.last_message_seq);
        const localSeq = sequenceRef.current.get(id);
        if (localSeq === undefined) sequenceRef.current.set(id, Math.max(normalizeNumber(conversation.joined_seq), normalizeNumber(conversation.last_read_seq)));
        if (resync && serverSeq > normalizeNumber(sequenceRef.current.get(id))) {
          pendingSequenceRef.current.set(id, Math.max(normalizeNumber(pendingSequenceRef.current.get(id)), serverSeq));
        }
        const targetSeq = normalizeNumber(pendingSequenceRef.current.get(id));
        const currentSeq = normalizeNumber(sequenceRef.current.get(id));
        if (targetSeq > currentSeq) syncConversationAfter(id, currentSeq);
      });
    } catch (error) {
      if (!handleAuthenticationError(error)) {
        setConversationNotice({ status: 'danger', title: '会话加载失败', description: error.message });
      }
    } finally {
      if (requestId === conversationRequestRef.current) {
        setConversationLoading(false);
        setConversationLoadingMore(false);
      }
    }
  }

  function updateLocalMessage(conversationId, clientMessageId, updater) {
    if (!conversationId || !clientMessageId) return;
    updateMessageState(conversationId, (current) => ({
      ...current,
      items: current.items.map((message) => message.client_message_id === clientMessageId && message.local ? updater(message) : message),
    }));
  }

  function clearMessageTimer(clientMessageId) {
    const timer = messageTimersRef.current.get(clientMessageId);
    if (timer) window.clearTimeout(timer);
    messageTimersRef.current.delete(clientMessageId);
  }

  function scheduleUnknownMessage(conversationId, clientMessageId) {
    clearMessageTimer(clientMessageId);
    const timer = window.setTimeout(() => {
      updateLocalMessage(conversationId, clientMessageId, (message) => (
        message.status === 'failed' ? message : { ...message, status: 'unknown' }
      ));
      messageTimersRef.current.delete(clientMessageId);
    }, 20_000);
    messageTimersRef.current.set(clientMessageId, timer);
  }

  function handleCreatedMessage(message) {
    if (!message?.conversation_id || !message?.seq) return;
    const id = normalizeNumber(message.conversation_id);
    const seq = normalizeNumber(message.seq);
    pendingSequenceRef.current.set(id, Math.max(normalizeNumber(pendingSequenceRef.current.get(id)), seq));
    clearMessageTimer(message.client_message_id);
    mergeConversationMessage(id, message);
    const active = isConversationVisible(id);
    updateConversationFromMessage(message, active);

    let currentSeq = sequenceRef.current.get(id);
    if (currentSeq === undefined) {
      const conversation = conversationItemsRef.current.find((item) => normalizeNumber(item.id) === id);
      if (!conversation) {
        refreshConversation(id);
        return;
      }
      currentSeq = Math.max(normalizeNumber(conversation.joined_seq), normalizeNumber(conversation.last_read_seq));
      sequenceRef.current.set(id, currentSeq);
    }

    if (seq === currentSeq + 1) {
      sequenceRef.current.set(id, seq);
      if (normalizeNumber(pendingSequenceRef.current.get(id)) <= seq) pendingSequenceRef.current.delete(id);
      reportProgress('conversation.delivered', id, seq);
      if (active) reportProgress('conversation.read', id, seq);
      const pendingSeq = normalizeNumber(pendingSequenceRef.current.get(id));
      if (pendingSeq > seq) syncConversationAfter(id, seq);
    } else if (seq > currentSeq + 1) {
      syncConversationAfter(id, currentSeq);
    } else if (normalizeNumber(pendingSequenceRef.current.get(id)) <= currentSeq) {
      pendingSequenceRef.current.delete(id);
    }
  }

  function handleProgressEvent(type, data) {
    if (!data?.conversation_id || !data?.user_id || !data?.seq) return;
    const id = String(data.conversation_id);
    const userId = String(data.user_id);
    const key = type === 'conversation.read' ? 'read' : 'delivered';
    setProgressByConversation((current) => ({
      ...current,
      [id]: {
        ...(current[id] || {}),
        [userId]: {
          ...(current[id]?.[userId] || {}),
          [key]: Math.max(normalizeNumber(current[id]?.[userId]?.[key]), normalizeNumber(data.seq)),
        },
      },
    }));
    if (key === 'read' && normalizeNumber(data.user_id) === normalizeNumber(profileRef.current?.id)) {
      setConversationItems((current) => current.map((item) => normalizeNumber(item.id) === normalizeNumber(data.conversation_id) ? {
        ...item,
        last_read_seq: Math.max(normalizeNumber(item.last_read_seq), normalizeNumber(data.seq)),
        unread_count: 0,
      } : item));
    }
  }

  function handleSocketEvent(event) {
    const data = event?.data || {};
    switch (event?.type) {
      case 'message.accepted':
        updateLocalMessage(data.conversation_id, data.client_message_id, (message) => ({ ...message, status: 'accepted' }));
        break;
      case 'message.created':
        handleCreatedMessage(data);
        break;
      case 'message.rejected':
        clearMessageTimer(data.client_message_id);
        updateLocalMessage(data.conversation_id, data.client_message_id, (message) => ({
          ...message,
          status: 'failed',
          error_code: data.code,
          error_message: data.message || '消息被拒绝',
        }));
        break;
      case 'conversation.delivered':
      case 'conversation.read':
        handleProgressEvent(event.type, data);
        break;
      case 'error': {
        const code = event.error?.code || 'REQUEST_FAILED';
        if (data.client_message_id) {
          clearMessageTimer(data.client_message_id);
          updateLocalMessage(data.conversation_id, data.client_message_id, (message) => ({
            ...message,
            status: code === 'MESSAGE_UNAVAILABLE' ? 'unknown' : 'failed',
            error_code: code,
            error_message: event.error?.message || '消息发送失败',
          }));
        } else {
          setConversationNotice({ status: 'danger', title: '实时操作失败', description: event.error?.message || '请稍后重试。' });
        }
        if (data.conversation_id && ['MESSAGE_NOT_FOUND', 'MESSAGE_PROGRESS_UNAVAILABLE', 'CONVERSATION_NOT_FOUND'].includes(code)) {
          reportedDeliveredRef.current.delete(normalizeNumber(data.conversation_id));
          reportedReadRef.current.delete(normalizeNumber(data.conversation_id));
        }
        if (code === 'CONVERSATION_NOT_FOUND') loadConversationPage();
        if (code === 'MESSAGE_NOT_FOUND' && data.conversation_id) refreshSelectedConversation(data.conversation_id);
        if (code === 'MESSAGE_PROGRESS_UNAVAILABLE' && data.conversation_id) {
          window.setTimeout(() => {
            const id = normalizeNumber(data.conversation_id);
            const seq = sequenceRef.current.get(id);
            if (seq) {
              reportProgress('conversation.delivered', id, seq);
              if (isConversationVisible(id)) reportProgress('conversation.read', id, seq);
            }
          }, 1500);
        }
        break;
      }
      default:
        break;
    }
  }

  socketEventRef.current = handleSocketEvent;

  useEffect(() => {
    const socket = new ChatSocket({
      onEvent: (event) => socketEventRef.current?.(event),
      onState: (next) => {
        connectionRef.current = next;
        setConnection(next);
        if (next.status === 'online') {
          const reconnecting = hadOnlineConnectionRef.current;
          hadOnlineConnectionRef.current = true;
          reportedDeliveredRef.current.clear();
          reportedReadRef.current.clear();
          loadConversationPage({ resync: reconnecting });
          const selectedId = selectedConversationRef.current;
          const seq = sequenceRef.current.get(normalizeNumber(selectedId));
          if (selectedId && seq) {
            reportProgress('conversation.delivered', selectedId, seq);
            if (isConversationVisible(selectedId)) reportProgress('conversation.read', selectedId, seq);
          }
        }
      },
      onAuthenticationFailure: (error) => handleAuthenticationError(error),
    });
    socketRef.current = socket;
    loadConversationPage();
    socket.start();
    return () => {
      socket.stop();
      socketRef.current = null;
      messageTimersRef.current.forEach((timer) => window.clearTimeout(timer));
      messageTimersRef.current.clear();
      syncRetryTimersRef.current.forEach((timer) => window.clearTimeout(timer));
      syncRetryTimersRef.current.clear();
    };
  }, []);

  useEffect(() => {
    const reportVisibleConversation = () => {
      if (document.visibilityState !== 'visible') return;
      const id = normalizeNumber(selectedConversationRef.current);
      const seq = sequenceRef.current.get(id);
      if (id && seq && isConversationVisible(id)) reportProgress('conversation.read', id, seq);
    };
    document.addEventListener('visibilitychange', reportVisibleConversation);
    window.addEventListener('focus', reportVisibleConversation);
    return () => {
      document.removeEventListener('visibilitychange', reportVisibleConversation);
      window.removeEventListener('focus', reportVisibleConversation);
    };
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => {
      if (connectionRef.current.status === 'online') loadConversationPage({ resync: true });
    }, 10 * 60_000);
    return () => window.clearInterval(timer);
  }, []);

  async function openConversation(conversationOrId) {
    const id = normalizeNumber(conversationOrId?.id || conversationOrId);
    if (!id) return;
    setSelectedConversationId(id);
    selectedConversationRef.current = id;
    chatAtBottomRef.current = false;
    setComposerText('');
    clearComposerAttachments();
    setView('chats');
    viewRef.current = 'chats';
    updateMessageState(id, (current) => ({ ...current, loading: true, error: null }));
    try {
      const [conversation, payload] = await Promise.all([
        getConversation(id),
        listMessages(id, { limit: 100 }),
      ]);
      replaceConversation(conversation);
      const items = Array.isArray(payload.items) ? payload.items : [];
      updateMessageState(id, (current) => ({
        ...current,
        items: mergeMessages(current.items, items),
        loading: false,
        initialized: true,
        hasOlder: Boolean(payload.has_more),
        nextBeforeSeq: payload.next_before_seq ?? null,
        error: null,
      }));
      const baselineSeq = Math.max(
        normalizeNumber(sequenceRef.current.get(id)),
        normalizeNumber(conversation.joined_seq),
        normalizeNumber(conversation.last_read_seq),
      );
      // A no-cursor request is the API's latest-page snapshot. Older history stays
      // pageable with before_seq, while this snapshot becomes the live-sync checkpoint.
      const snapshotSeq = items.length
        ? Math.max(...items.map((item) => normalizeNumber(item.seq)))
        : baselineSeq;
      const finalSeq = Math.max(baselineSeq, snapshotSeq);
      sequenceRef.current.set(id, finalSeq);
      const targetSeq = Math.max(
        normalizeNumber(pendingSequenceRef.current.get(id)),
        normalizeNumber(conversation.last_message_seq),
      );
      if (targetSeq > finalSeq) {
        pendingSequenceRef.current.set(id, targetSeq);
        syncConversationAfter(id, finalSeq);
      } else {
        pendingSequenceRef.current.delete(id);
      }
      if (finalSeq > normalizeNumber(conversation.joined_seq)) {
        reportProgress('conversation.delivered', id, finalSeq);
        if (isConversationVisible(id)) reportProgress('conversation.read', id, finalSeq);
      }
    } catch (error) {
      if (!handleAuthenticationError(error)) {
        updateMessageState(id, (current) => ({ ...current, loading: false, error: error.message || '会话加载失败' }));
      }
    }
  }

  function refreshSelectedConversation(conversationId = selectedConversationRef.current) {
    if (conversationId) return openConversation(conversationId);
    return Promise.resolve();
  }

  async function loadOlderMessages() {
    const id = normalizeNumber(selectedConversationRef.current);
    const current = messageStates[String(id)];
    if (!id || !current?.hasOlder || current.loadingOlder || current.nextBeforeSeq === null) return;
    updateMessageState(id, (value) => ({ ...value, loadingOlder: true, error: null }));
    try {
      const payload = await listMessages(id, { beforeSeq: current.nextBeforeSeq, limit: 100 });
      updateMessageState(id, (value) => ({
        ...value,
        items: mergeMessages(value.items, payload.items || []),
        loadingOlder: false,
        hasOlder: Boolean(payload.has_more),
        nextBeforeSeq: payload.next_before_seq ?? null,
      }));
    } catch (error) {
      if (!handleAuthenticationError(error)) {
        updateMessageState(id, (value) => ({ ...value, loadingOlder: false, error: error.message || '历史消息加载失败' }));
      }
    }
  }

  function refreshMessageAttachments(message) {
    const conversationId = normalizeNumber(message?.conversation_id);
    const seq = normalizeNumber(message?.seq);
    if (!conversationId || !seq) return Promise.resolve(null);
    const key = `${conversationId}:${seq}`;
    if (mediaRefreshesRef.current.has(key)) return mediaRefreshesRef.current.get(key);

    const task = listMessages(conversationId, { beforeSeq: seq + 1, limit: 100 })
      .then((payload) => {
        const refreshed = (payload.items || []).find((item) => normalizeNumber(item.seq) === seq);
        if (refreshed) mergeConversationMessage(conversationId, refreshed);
        return refreshed || null;
      })
      .catch((error) => {
        handleAuthenticationError(error);
        return null;
      })
      .finally(() => mediaRefreshesRef.current.delete(key));
    mediaRefreshesRef.current.set(key, task);
    return task;
  }

  async function handleComposerFiles(fileList) {
    const files = Array.from(fileList || []);
    if (!files.length) return;
    const available = Math.max(0, 6 - composerAttachmentsRef.current.length);
    const selected = files.slice(0, available);
    if (selected.length < files.length) setConversationNotice({ status: 'danger', title: '附件过多', description: '一次最多选择 6 个附件。' });

    const valid = selected.map((file) => {
      const key = createClientMessageId();
      const filenameBytes = new TextEncoder().encode(file.name).length;
      let error = '';
      if (!file.name || filenameBytes > 255 || /[/\\]/.test(file.name)) error = '文件名不符合要求';
      else if (!file.type || file.type.includes(';')) error = '无法识别文件格式';
      else if (file.size < 1 || file.size > 33_554_432) error = '文件需小于 32 MiB';
      return {
        key,
        file,
        filename: file.name,
        mime_type: file.type,
        size: file.size,
        type: file.type.startsWith('image/') ? 'image' : file.type.startsWith('video/') ? 'video' : file.type.startsWith('audio/') ? 'audio' : 'file',
        preview_url: registerPreviewURL(file),
        status: error ? 'failed' : 'uploading',
        error,
      };
    });
    updateComposerAttachments((current) => [...current, ...valid]);

    await Promise.all(valid.filter((item) => item.status === 'uploading').map(async (item) => {
      try {
        const requested = await requestMediaUpload(item.file);
        await uploadMediaFile(item.file, requested.upload);
        const completed = await completeMediaUpload(requested.media_id);
        updateComposerAttachments((current) => current.map((candidate) => candidate.key === item.key ? {
          ...candidate,
          status: 'ready',
          media_id: completed.media_id || requested.media_id,
          type: completed.type || candidate.type,
          filename: completed.filename || candidate.filename,
          mime_type: completed.mime_type || candidate.mime_type,
          size: completed.size || candidate.size,
          url: completed.url || candidate.url,
          url_expired_at: completed.url_expired_at || candidate.url_expired_at,
          media: completed,
        } : candidate));
      } catch (error) {
        if (!handleAuthenticationError(error)) {
          updateComposerAttachments((current) => current.map((candidate) => candidate.key === item.key ? {
            ...candidate,
            status: 'failed',
            error: error.message || '上传失败',
          } : candidate));
        }
      }
    }));
  }

  function removeComposerAttachment(key) {
    const attachment = composerAttachmentsRef.current.find((item) => item.key === key);
    releasePreviewURL(attachment?.preview_url);
    updateComposerAttachments((current) => current.filter((item) => item.key !== key));
  }

  function dispatchMessage(message) {
    const sent = socketRef.current?.send('message.send', {
      client_message_id: message.client_message_id,
      conversation_id: message.conversation_id,
      text: message.text,
      media_ids: (message.local_attachments || []).map((item) => normalizeNumber(item.media_id)),
    });
    if (sent) scheduleUnknownMessage(message.conversation_id, message.client_message_id);
    else updateLocalMessage(message.conversation_id, message.client_message_id, (current) => ({
      ...current,
      status: 'failed',
      error_message: '实时连接尚未就绪',
    }));
  }

  function handleSendMessage(event) {
    event?.preventDefault?.();
    if (sendGuardRef.current) return;
    const id = normalizeNumber(selectedConversationRef.current);
    const text = composerText.trim();
    const readyAttachments = composerAttachments.filter((item) => item.status === 'ready');
    if (!id || (!text && !readyAttachments.length) || connection.status !== 'online') return;
    sendGuardRef.current = true;
    const clientMessageId = createClientMessageId();
    const localMessage = {
      client_message_id: clientMessageId,
      conversation_id: id,
      sender_id: normalizeNumber(profileRef.current?.id || session.user?.id),
      type: 1,
      text,
      attachments: [],
      local_attachments: readyAttachments.map((item) => ({
        key: item.key,
        media_id: item.media_id,
        type: item.type,
        filename: item.filename || item.file.name,
        mime_type: item.mime_type || item.file.type,
        size: item.size || item.file.size,
        url: item.url || item.media?.url || '',
        url_expired_at: item.url_expired_at || item.media?.url_expired_at,
        local_preview_url: item.preview_url,
      })),
      local: true,
      local_order: Date.now(),
      local_created_at: new Date().toISOString(),
      status: 'pending',
    };
    const localPreviewURLs = localMessage.local_attachments.map((attachment) => attachment.local_preview_url).filter(Boolean);
    if (localPreviewURLs.length) localMessagePreviewURLsRef.current.set(clientMessageId, localPreviewURLs);
    mergeConversationMessage(id, localMessage);
    setComposerText('');
    updateComposerAttachments((current) => current.filter((item) => item.status !== 'ready'));
    dispatchMessage(localMessage);
    window.setTimeout(() => { sendGuardRef.current = false; }, 0);
  }

  function retryMessage(message) {
    updateLocalMessage(message.conversation_id, message.client_message_id, (current) => ({
      ...current,
      status: 'pending',
      error_code: undefined,
      error_message: undefined,
    }));
    dispatchMessage(message);
  }

  function handleChatReadVisibility(isAtBottom) {
    chatAtBottomRef.current = Boolean(isAtBottom);
    if (!isAtBottom) return;
    const id = normalizeNumber(selectedConversationRef.current);
    const seq = normalizeNumber(sequenceRef.current.get(id));
    if (id && seq && isConversationVisible(id)) reportProgress('conversation.read', id, seq);
  }

  async function handleConversationCreated(result) {
    setShowConversationDialog(false);
    const conversation = await refreshConversation(result.id);
    if (conversation) openConversation(conversation);
  }

  async function startDirectChat(user) {
    setStartingChatUserId(user.id);
    setConversationNotice(null);
    try {
      const result = await createDirectConversation(user.id);
      setSelectedUser(null);
      await handleConversationCreated(result);
    } catch (error) {
      if (!handleAuthenticationError(error)) setNotice({ status: 'danger', title: '私聊创建失败', description: error.message });
    } finally {
      setStartingChatUserId(null);
    }
  }

  function fillProfile(value) {
    setProfile(value);
    setDraft({
      nickname: value.nickname || '',
      gender: Number(value.gender) || 0,
      signature: value.signature || '',
      birthday: value.birthday || '',
      country: value.country || '',
      province: value.province || '',
    });
  }

  async function loadProfile() {
    setLoading(true);
    setNotice(null);
    try { fillProfile(await getMyProfile()); }
    catch (error) {
      if (handleAuthenticationError(error)) return;
      setNotice({ status: 'danger', title: '资料加载失败', description: error.message });
    } finally { setLoading(false); }
  }

  useEffect(() => { loadProfile(); }, []);

  useEffect(() => {
    if (!profile?.avatar_url || !profile?.url_expired_at) return undefined;
    const expiresAt = new Date(profile.url_expired_at).getTime();
    if (!Number.isFinite(expiresAt)) return undefined;
    const delay = Math.max(0, expiresAt - Date.now() - 60_000);
    const timer = window.setTimeout(async () => {
      try {
        const refreshed = await getMyProfile();
        setProfile((current) => current ? {
          ...current,
          avatar_url: refreshed.avatar_url || '',
          url_expired_at: refreshed.url_expired_at || '',
        } : current);
      } catch (error) {
        handleAuthenticationError(error);
      }
    }, Math.min(delay, 2_147_000_000));
    return () => window.clearTimeout(timer);
  }, [profile?.avatar_url, profile?.url_expired_at, onSignedOut]);

  useLayoutEffect(() => {
    if (contextRef.current) {
      contextRef.current.scrollTop = 0;
      const panelScroller = contextRef.current.querySelector('.reference-profile-content, .contact-search-scroll');
      if (panelScroller) panelScroller.scrollTop = 0;
    }
  }, [view, editingProfile]);

  const navigate = (nextView) => {
    viewRef.current = nextView;
    if (nextView !== 'chats' && nextView !== 'groups') chatAtBottomRef.current = false;
    setView(nextView);
    setNotice(null);
    setErrors({});
    setEditingProfile(false);
    setSelectedUser(null);
    if (nextView === 'chats' || nextView === 'groups') loadConversationPage({ resync: true });
  };
  const updateDraft = (field, value) => { setDraft((current) => ({ ...current, [field]: value })); setErrors((current) => ({ ...current, [field]: undefined })); };
  const updateCountry = (country) => {
    setDraft((current) => ({ ...current, country, province: '' }));
    setErrors((current) => ({ ...current, country: undefined, province: undefined }));
  };
  const cancelEdit = () => { fillProfile(profile); setErrors({}); setNotice(null); setEditingProfile(false); };

  const handleAvatarFile = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;

    const filenameBytes = new TextEncoder().encode(file.name).length;
    if (!file.type.startsWith('image/')) {
      setNotice({ status: 'danger', title: '无法上传头像', description: '请选择图片文件。' });
      return;
    }
    if (file.size < 1 || file.size > 33_554_432) {
      setNotice({ status: 'danger', title: '无法上传头像', description: '图片大小必须在 32 MiB 以内。' });
      return;
    }
    if (!file.name || filenameBytes > 255 || /[/\\]/.test(file.name)) {
      setNotice({ status: 'danger', title: '无法上传头像', description: '图片文件名不符合要求。' });
      return;
    }

    setAvatarUploading(true);
    setNotice(null);
    try {
      const requested = await requestMediaUpload(file);
      await uploadMediaFile(file, requested.upload);
      await completeMediaUpload(requested.media_id);
      const avatar = await updateMyAvatar(requested.media_id);
      setProfile((current) => current ? { ...current, ...avatar } : current);
      setNotice({ status: 'success', title: '头像已更新', description: '新头像已经保存。' });
    } catch (error) {
      if (handleAuthenticationError(error)) return;
      setNotice({ status: 'danger', title: '头像更新失败', description: error.message || '请稍后重试。' });
    } finally {
      setAvatarUploading(false);
    }
  };

  const invalidateUserSearch = () => {
    searchRequestRef.current += 1;
    setSearchResults([]);
    setSearchMeta({ next_cursor: null, has_more: false });
    setLastSearch(null);
    setSearching(false);
    setLoadingMore(false);
    setNotice(null);
  };
  const updateSearchFilter = (field, value) => {
    invalidateUserSearch();
    setSearchFilters((current) => ({ ...current, [field]: value }));
    setSearchErrors((current) => ({ ...current, [field]: undefined }));
  };
  const updateSearchCountry = (country) => {
    invalidateUserSearch();
    setSearchFilters((current) => ({ ...current, country, province: '' }));
    setSearchErrors((current) => ({ ...current, country: undefined, province: undefined }));
  };

  const changeSearchMode = (mode) => {
    invalidateUserSearch();
    setSearchMode(mode);
    setSearchErrors({});
    setSelectedUser(null);
    setNotice(null);
  };

  const handleSave = async (event) => {
    event.preventDefault();
    const nickname = draft.nickname.trim().replace(/\s+/g, ' ');
    const signature = draft.signature.trim();
    const birthday = draft.birthday.trim();
    const country = draft.country.trim().toUpperCase();
    const province = draft.province.trim();
    const gender = Number(draft.gender);
    const nextErrors = {};
    if (!nickname || Array.from(nickname).length > 50) nextErrors.nickname = '昵称需为 1–50 个字符';
    if (Array.from(signature).length > 200) nextErrors.signature = '个性签名不能超过 200 个字符';
    if (![0, 1, 2].includes(gender)) nextErrors.gender = '请选择正确的性别';
    if (birthday && (!isCalendarDate(birthday) || birthday > todayValue())) nextErrors.birthday = '请输入不晚于今天的正确日期';
    if (!birthday && profile.birthday) nextErrors.birthday = '当前接口暂不支持清空生日';
    if (country && !/^[A-Z]{2}$/.test(country)) nextErrors.country = '请输入两个大写字母，例如 CN';
    if (Array.from(province).length > 100) nextErrors.province = '省份或地区不能超过 100 个字符';
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length) return;

    const changes = {};
    if (nickname !== (profile.nickname || '')) changes.nickname = nickname;
    if (gender !== Number(profile.gender || 0)) changes.gender = gender;
    if (signature !== (profile.signature || '')) changes.signature = signature;
    if (birthday && birthday !== (profile.birthday || '')) changes.birthday = birthday;
    if (country !== (profile.country || '')) changes.country = country;
    if (province !== (profile.province || '')) changes.province = province;

    if (!Object.keys(changes).length) {
      setEditingProfile(false);
      setNotice({ status: 'success', title: '资料未变更', description: '当前资料已经是最新状态。' });
      return;
    }

    setSaving(true);
    setNotice(null);
    try {
      fillProfile(await updateMyProfile(changes));
      setEditingProfile(false);
      setNotice({ status: 'success', title: '保存成功', description: '个人资料已经更新。' });
    } catch (error) {
      if (handleAuthenticationError(error)) return;
      const fields = { INVALID_NICKNAME: 'nickname', INVALID_GENDER: 'gender', INVALID_SIGNATURE: 'signature', INVALID_BIRTHDAY: 'birthday', INVALID_COUNTRY: 'country', INVALID_PROVINCE: 'province' };
      if (fields[error.code]) setErrors((current) => ({ ...current, [fields[error.code]]: error.message }));
      setNotice({ status: 'danger', title: '保存失败', description: error.message });
    } finally { setSaving(false); }
  };

  const handleSearch = async (event, append = false) => {
    event?.preventDefault();
    const requestId = ++searchRequestRef.current;
    setNotice(null);
    const nextErrors = {};
    let nextFilters;

    if (append) {
      if (!lastSearch || !searchMeta.has_more || searchMeta.next_cursor === null) return;
      nextFilters = { ...lastSearch, cursor: searchMeta.next_cursor };
      setLoadingMore(true);
    } else if (searchMode === 'phone') {
      const phone = searchFilters.phone.trim();
      if (!phonePattern.test(phone)) nextErrors.phone = '请输入正确的 11 位大陆手机号';
      nextFilters = { phone, limit: 20 };
    } else {
      const nickname = searchFilters.nickname.trim().replace(/\s+/g, ' ');
      const country = searchFilters.country.trim().toUpperCase();
      const province = searchFilters.province.trim();
      const ageText = searchFilters.age.trim();
      const gender = searchFilters.gender;

      if (nickname && Array.from(nickname).length > 50) nextErrors.nickname = '昵称不能超过 50 个字符';
      if (country && !/^[A-Z]{2}$/.test(country)) nextErrors.country = '请输入两个大写字母，例如 CN';
      if (Array.from(province).length > 100) nextErrors.province = '省份或地区不能超过 100 个字符';
      if (ageText && (!/^\d+$/.test(ageText) || Number(ageText) > 150)) nextErrors.age = '年龄需为 0–150 的整数';
      if (!nickname && !country && !province && ageText === '' && gender === '') nextErrors.form = '请至少填写一个搜索条件';

      nextFilters = { nickname, country, province, age: ageText, gender, limit: 20 };
    }

    setSearchErrors(nextErrors);
    if (Object.keys(nextErrors).length) {
      setSearching(false);
      setLoadingMore(false);
      return;
    }

    if (!append) {
      setSearching(true);
      setSelectedUser(null);
      setSearchResults([]);
      setSearchMeta({ next_cursor: null, has_more: false });
      setLastSearch(nextFilters);
    }

    try {
      const result = await searchUsers(nextFilters);
      if (requestId !== searchRequestRef.current) return;
      setSearchResults((current) => {
        const next = append ? [...current] : [];
        result.users.forEach((user) => {
          const index = next.findIndex((item) => normalizeNumber(item.id) === normalizeNumber(user.id));
          if (index >= 0) next[index] = user;
          else next.push(user);
        });
        return next;
      });
      setSearchMeta(result.meta);
    } catch (error) {
      if (requestId !== searchRequestRef.current) return;
      if (handleAuthenticationError(error)) return;
      const fields = { INVALID_PHONE: 'phone', INVALID_NICKNAME: 'nickname', INVALID_COUNTRY: 'country', INVALID_PROVINCE: 'province', INVALID_GENDER: 'gender' };
      if (fields[error.code]) setSearchErrors((current) => ({ ...current, [fields[error.code]]: error.message }));
      setNotice({ status: 'danger', title: '搜索失败', description: error.message });
    } finally {
      if (requestId === searchRequestRef.current) {
        setSearching(false);
        setLoadingMore(false);
      }
    }
  };

  const handleLogout = async () => {
    setLoggingOut(true);
    try { await logoutAccount(); } finally { setLoggingOut(false); onSignedOut(); }
  };

  const navItems = [
    { id: 'chats', label: '消息', icon: 'chats' },
    { id: 'contacts', label: '联系人', icon: 'contacts' },
    { id: 'calls', label: '通话', icon: 'calls' },
    { id: 'groups', label: '群组', icon: 'groups' },
    { id: 'profile', label: '资料', icon: 'profile' },
  ];

  const renderConversationPanel = (groupsOnly = false) => <>
    <header className="context-head"><div><h1>{groupsOnly ? '群组' : '消息'}</h1><small className="context-subtitle">{connectionCopy(connection.status)}</small></div><button className="square-button" type="button" aria-label={groupsOnly ? '新建群聊' : '新建会话'} onClick={() => { setConversationDialogMode(groupsOnly ? 'group' : 'direct'); setShowConversationDialog(true); }}><Icon name="plus" /></button></header>
    <label className="context-search"><Icon name="search" size={18} /><input value={conversationQuery} onChange={(event) => setConversationQuery(event.target.value)} placeholder={groupsOnly ? '搜索群组…' : '搜索会话…'} /></label>
    <div className="conversation-scroll">
      <Notice notice={conversationNotice} />
      {conversationLoading && conversationItems.length === 0 && <div className="conversation-list-state"><Spinner /><span>正在加载会话…</span></div>}
      {!conversationLoading && visibleConversations.length === 0 && !conversationNotice && <div className="conversation-list-state"><Icon name={groupsOnly ? 'groups' : 'chats'} size={31} /><strong>{conversationQuery ? '没有匹配的会话' : groupsOnly ? '还没有群聊' : '还没有会话'}</strong><span>{conversationQuery ? '换个关键词试试。' : '点击右上角的加号开始聊天。'}</span></div>}
      {visibleConversations.length > 0 && <><h2>{groupsOnly ? '我的群聊' : '最近会话'}</h2>{visibleConversations.map((conversation) => {
        const preview = conversation.last_message
          ? conversation.last_message.text || '[附件消息]'
          : '会话已创建，开始聊天吧';
        const unread = normalizeNumber(conversation.unread_count);
        return <button className={`conversation-item${normalizeNumber(selectedConversationId) === normalizeNumber(conversation.id) ? ' active' : ''}`} type="button" key={conversation.id} onClick={() => openConversation(conversation)}>
          <UserAvatar profile={conversationAvatarProfile(conversation)} />
          <span><strong>{conversationTitle(conversation)}</strong><small>{preview}</small></span>
          <time>{formatConversationTime(conversation.last_message?.created_at)}</time>
          {unread > 0 && <em>{unread > 99 ? '99+' : unread}</em>}
        </button>;
      })}</>}
      {conversationMeta.has_more && <button className="load-more-conversations" type="button" onClick={() => loadConversationPage({ append: true })} disabled={conversationLoadingMore}>{conversationLoadingMore ? <><Spinner />加载中</> : '加载更多会话'}</button>}
    </div>
  </>;

  const renderPanel = () => {
    if (loading) return <div className="native-panel-state"><Spinner /><p>正在加载…</p></div>;
    if (!profile) return <div className="native-panel-state"><Notice notice={notice} /><button className="native-primary-button" onClick={loadProfile}>重新加载</button></div>;

    if (view === 'chats') return renderConversationPanel(false);

    if (view === 'groups') return renderConversationPanel(true);

    if (view === 'contacts' && selectedUser) return <ContactDetail user={selectedUser} onBack={() => setSelectedUser(null)} onStartChat={startDirectChat} startingChat={normalizeNumber(startingChatUserId) === normalizeNumber(selectedUser.id)} />;

    if (view === 'contacts') return <>
      <header className="context-head"><h1>联系人</h1><Icon name="contacts" /></header>
      <div className="contact-search-scroll">
        <div className="contact-search-tabs" role="tablist" aria-label="用户搜索方式"><button type="button" role="tab" aria-selected={searchMode === 'profile'} className={searchMode === 'profile' ? 'active' : ''} onClick={() => changeSearchMode('profile')}>组合条件</button><button type="button" role="tab" aria-selected={searchMode === 'phone'} className={searchMode === 'phone' ? 'active' : ''} onClick={() => changeSearchMode('phone')}>手机号</button></div>
        <form className="contact-search-form" onSubmit={handleSearch}>
          {searchMode === 'phone' ? <SearchField label="手机号" icon="phone" error={searchErrors.phone} className="contact-field-wide"><input aria-label="手机号" value={searchFilters.phone} onChange={(event) => updateSearchFilter('phone', event.target.value.replace(/\D/g, '').slice(0, 11))} inputMode="tel" placeholder="手机号" /></SearchField> : <>
            <SearchField label="昵称" icon="profile" error={searchErrors.nickname} className="contact-field-wide"><input aria-label="昵称" value={searchFilters.nickname} onChange={(event) => updateSearchFilter('nickname', event.target.value)} maxLength={50} placeholder="昵称" /></SearchField>
            <div className="contact-filter-grid">
              <SearchField label="国家/地区" icon="globe" error={searchErrors.country}><select aria-label="国家/地区" value={searchFilters.country} onChange={(event) => updateSearchCountry(event.target.value)}><option value="">国家/地区不限</option>{countryOptions.map((item) => <option value={item.value} key={item.value}>{item.label}</option>)}</select></SearchField>
              <SearchField label="省份、州或地区" icon="location" error={searchErrors.province}><select aria-label="省份、州或地区" value={searchFilters.province} onChange={(event) => updateSearchFilter('province', event.target.value)} disabled={!searchFilters.country || searchProvinceOptions.length === 0}><option value="">{searchFilters.country && searchProvinceOptions.length === 0 ? '暂无可选行政区' : searchFilters.country ? '省份/地区不限' : '请先选择国家'}</option>{searchProvinceOptions.map((item) => <option value={item.value} key={`${searchFilters.country}-${item.value}`}>{item.label}</option>)}</select></SearchField>
              <SearchField label="年龄" icon="cake" error={searchErrors.age}><input aria-label="年龄" value={searchFilters.age} onChange={(event) => updateSearchFilter('age', event.target.value.replace(/\D/g, '').slice(0, 3))} inputMode="numeric" placeholder="年龄" /></SearchField>
              <SearchField label="性别" icon="profile" error={searchErrors.gender}><select aria-label="性别" value={searchFilters.gender} onChange={(event) => updateSearchFilter('gender', event.target.value)}><option value="">不限</option>{genderOptions.map((item) => <option value={item.value} key={item.value}>{item.label}</option>)}</select></SearchField>
            </div>
          </>}
          {searchErrors.form && <small className="contact-form-error">{searchErrors.form}</small>}
          <button className="contact-search-submit" type="submit" disabled={searching}>{searching ? <Spinner /> : <Icon name="search" size={17} />}{searching ? '搜索中' : '搜索用户'}</button>
        </form>
        <Notice notice={notice} />
        {!lastSearch && !searching && !notice && <div className="panel-empty contact-empty"><Icon name="contacts" size={34} /><p>可以按昵称、地区、年龄或手机号查找伙伴。</p></div>}
        {lastSearch && !searching && !notice && searchResults.length === 0 && <div className="panel-empty contact-empty"><Icon name="search" size={31} /><p>没有找到符合条件的用户，换个条件试试。</p></div>}
        {searchResults.length > 0 && <div className="contact-result-list">{searchResults.map((user, index) => <SearchResult user={user} onOpen={setSelectedUser} key={user.id || `${user.nickname}-${user.birthday || ''}-${index}`} />)}{searchMeta.has_more && <button className="load-more-users" type="button" onClick={() => handleSearch(null, true)} disabled={loadingMore}>{loadingMore ? <><Spinner />加载中</> : '加载更多'}</button>}</div>}
      </div>
    </>;

    if (view === 'profile') return <>
      <header className="context-head"><h1>个人资料</h1>{!editingProfile && <button className="icon-button" onClick={() => { setEditingProfile(true); setNotice(null); setErrors({}); }} aria-label="编辑资料"><Icon name="edit" size={18} /></button>}</header>
      <div className="reference-profile-cover" />
      <div className="reference-profile-content"><div className="reference-avatar-wrap"><UserAvatar profile={profile} size="xl" /><input ref={avatarInputRef} className="avatar-file-input" type="file" accept="image/*" hidden onChange={handleAvatarFile} /><button className="avatar-edit-trigger" type="button" onClick={() => avatarInputRef.current?.click()} disabled={avatarUploading} aria-label="更换头像">{avatarUploading ? <Spinner /> : <Icon name="camera" size={16} />}</button></div><h2>{profile.nickname}</h2><span>{connectionCopy(connection.status)}</span>
        {editingProfile ? <form className="reference-edit-form inline-profile-edit-form" onSubmit={handleSave}>
          <ProfileEditField label="个性签名" icon="edit" error={errors.signature} className="profile-edit-signature"><textarea aria-label="个性签名" value={draft.signature} onChange={(event) => updateDraft('signature', event.target.value.replace(/[\r\n]/g, ' '))} maxLength={200} rows={3} placeholder="写一句话介绍自己…" /></ProfileEditField>
          <ProfileEditField label="昵称" icon="profile" error={errors.nickname}><input aria-label="昵称" value={draft.nickname} onChange={(event) => updateDraft('nickname', event.target.value)} maxLength={50} placeholder="昵称" /></ProfileEditField>
          <ProfileEditField label="邮箱" icon="mail"><input aria-label="邮箱" value={profile.email || '未绑定'} readOnly /></ProfileEditField>
          <ProfileEditField label="手机号" icon="phone"><input aria-label="手机号" value={profile.phone} readOnly /></ProfileEditField>
          <ProfileEditField label="性别" icon="profile" error={errors.gender}><select aria-label="性别" value={draft.gender} onChange={(event) => updateDraft('gender', Number(event.target.value))}>{genderOptions.map((item) => <option value={item.value} key={item.value}>{item.label}</option>)}</select></ProfileEditField>
          <ProfileEditField label="生日" icon="calendar" error={errors.birthday}><input aria-label="生日" type="date" value={draft.birthday} max={todayValue()} onChange={(event) => updateDraft('birthday', event.target.value)} /></ProfileEditField>
          <ProfileEditField label="国家/地区" icon="globe" error={errors.country}><select aria-label="国家/地区" value={draft.country} onChange={(event) => updateCountry(event.target.value)}><option value="">未设置</option>{countryOptions.map((item) => <option value={item.value} key={item.value}>{item.label}</option>)}</select></ProfileEditField>
          <ProfileEditField label="省份、州或地区" icon="location" error={errors.province}><select aria-label="省份、州或地区" value={draft.province} onChange={(event) => updateDraft('province', event.target.value)} disabled={!draft.country || provinceOptions.length === 0}><option value="">{draft.country && provinceOptions.length === 0 ? '暂无可选行政区' : '未设置'}</option>{provinceOptions.map((item) => <option value={item.value} key={`${draft.country}-${item.value}`}>{item.label}</option>)}</select></ProfileEditField>
          <div className="reference-form-actions"><button className="save" type="submit" disabled={saving || avatarUploading}>{saving ? <><Spinner />保存中</> : '保存'}</button><button type="button" onClick={cancelEdit} disabled={avatarUploading}>取消</button></div>
        </form> : <><p>{profile.signature || '这个人还没有填写个性签名。'}</p><ProfileInfoList items={[
          { label: '手机号', icon: 'phone', value: profile.phone },
          { label: '邮箱', icon: 'mail', value: profile.email || '未绑定' },
          { label: '性别', icon: 'profile', value: genderLabel(profile.gender) },
          { label: '生日', icon: 'calendar', value: profile.birthday || '未设置' },
          { label: '地区', icon: 'location', value: regionLabel(profile) },
          { label: '加入时间', icon: 'clock', value: joinedDateLabel(profile.created_at) },
        ]} /></>}
        <Notice notice={notice} />
      </div>
    </>;

    return <><header className="context-head"><h1>通话</h1><Icon name="calls" /></header><div className="panel-empty"><Icon name="calls" size={38} /><h2>当前版本暂不支持通话</h2><p>API 尚未提供语音或视频通话能力；消息、附件与已读状态仍可正常使用。</p></div></>;
  };

  return <main className={`reference-app${selectedConversation && (view === 'chats' || view === 'groups') ? ' chat-open' : ''}`}>
    <aside className="reference-rail"><button className="reference-logo" onClick={() => navigate('chats')} aria-label="Mocha 首页"><Icon name="chats" size={28} /></button><nav aria-label="主要功能">{navItems.map((item) => <button key={item.id} className={view === item.id ? 'active' : ''} onClick={() => navigate(item.id)}><Icon name={item.icon} /><span>{item.label}</span></button>)}</nav><div className="rail-bottom"><button onClick={handleLogout} disabled={loggingOut}>{loggingOut ? <Spinner /> : <Icon name="logout" />}<span>退出</span></button><button className="rail-user" onClick={() => navigate('profile')} aria-label="打开个人资料"><UserAvatar profile={profile || session.user} size="sm" /></button></div></aside>
    <aside className="reference-context" ref={contextRef}>{renderPanel()}</aside>
    <ConversationArea
      conversation={(view === 'chats' || view === 'groups') ? selectedConversation : null}
      currentUser={profile || session.user}
      state={selectedConversation ? messageStates[String(selectedConversation.id)] : null}
      connection={connection}
      progress={selectedConversation ? progressByConversation[String(selectedConversation.id)] : null}
      composerText={composerText}
      attachments={composerAttachments}
      onComposerText={setComposerText}
      onFiles={handleComposerFiles}
      onRemoveAttachment={removeComposerAttachment}
      onSend={handleSendMessage}
      onRetry={retryMessage}
      onLoadOlder={loadOlderMessages}
      onRefreshAttachment={refreshMessageAttachments}
      onReleasePreviewURL={releasePreviewURL}
      onReadVisibilityChange={handleChatReadVisibility}
      onBack={() => { chatAtBottomRef.current = false; setSelectedConversationId(null); }}
      onReconnect={() => socketRef.current?.restart()}
    />
    {showConversationDialog && <NewConversationDialog initialMode={conversationDialogMode} onClose={() => setShowConversationDialog(false)} onCreated={handleConversationCreated} onAuthExpired={(message) => signedOutRef.current?.(message)} />}
  </main>;
}
