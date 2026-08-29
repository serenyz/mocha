import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { allCountries } from 'country-region-data';
import { logoutAccount } from '../api/auth';
import { completeMediaUpload, requestMediaUpload, uploadMediaFile } from '../api/media';
import { getMyProfile, searchUsers, updateMyAvatar, updateMyProfile } from '../api/users';

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

const conversations = [
  { name: '产品设计组', initial: '产', color: 'coral', preview: '新版界面已经同步', time: '刚刚', badge: 3, online: true },
  { name: '周远', initial: '周', color: 'blue', preview: '下午一起确认接口', time: '10:28', online: true },
  { name: '林晓', initial: '晓', color: 'violet', preview: '好的，收到啦', time: '昨天' },
  { name: '技术讨论组', initial: '技', color: 'green', preview: '后端服务已更新', time: '周一', badge: 6 },
  { name: '陈默', initial: '陈', color: 'amber', preview: '文件已经发给你了', time: '周日' },
];

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
  };
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name] || paths.chats}</svg>;
}

function resolveAvatarURL(profile) {
  const value = profile?.avatar_url || profile?.avata_url || '';
  if (!value) return '';
  if (/^(https?:|data:|blob:)/i.test(value)) return value;
  const base = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '');
  return `${base}${value.startsWith('/') ? value : `/${value}`}`;
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

function ContactDetail({ user, onBack }) {
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
    </div>
  </>;
}

function ConversationArea() {
  return <section className="reference-chat">
    <header className="reference-chat-head">
      <div className="chat-person"><DemoAvatar item={{ initial: '产', color: 'coral', online: true }} /><span><strong>产品设计组</strong><small>5 人在线</small></span></div>
      <div className="chat-head-actions"><button className="green" aria-label="语音通话"><Icon name="phone" /></button><button className="yellow" aria-label="视频通话"><Icon name="video" /></button><button aria-label="更多操作"><Icon name="more" /></button></div>
    </header>
    <div className="reference-message-list">
      <div className="message-row incoming"><DemoAvatar item={{ initial: '周', color: 'blue' }} size="sm" /><div><strong>周远</strong><p>登录与注册接口已经联调完成了。</p><time>10:18</time></div></div>
      <div className="message-row outgoing"><div><p>收到，我开始调整登录后的界面。</p><time>10:24 ✓✓</time></div></div>
      <div className="message-row incoming"><DemoAvatar item={{ initial: '晓', color: 'violet' }} size="sm" /><div><strong>林晓</strong><p>个人资料里记得使用用户返回的头像地址。</p><time>10:31</time></div></div>
      <div className="message-row outgoing"><div><p>好的，头像、资料查看和编辑状态都会统一。</p><time>10:36 ✓✓</time></div></div>
    </div>
    <footer className="reference-composer"><button aria-label="添加附件"><Icon name="attach" /></button><button aria-label="语音输入"><Icon name="mic" /></button><input placeholder="输入消息…" aria-label="输入消息" /><button className="send-button" aria-label="发送消息"><Icon name="send" /></button></footer>
  </section>;
}

export default function WorkspacePage({ session, onSignedOut }) {
  const contextRef = useRef(null);
  const avatarInputRef = useRef(null);
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
  const provinceOptions = useMemo(() => {
    const options = getProvinceOptions(draft.country);
    if (draft.province && !options.some((item) => item.value === draft.province)) return [{ value: draft.province, label: draft.province }, ...options];
    return options;
  }, [draft.country, draft.province]);
  const searchProvinceOptions = useMemo(() => getProvinceOptions(searchFilters.country), [searchFilters.country]);

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
      if (error.status === 401) return onSignedOut(error.message || '登录状态已失效，请重新登录。');
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
        if (error.status === 401) onSignedOut(error.message || '登录状态已失效，请重新登录。');
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

  const navigate = (nextView) => { setView(nextView); setNotice(null); setErrors({}); setEditingProfile(false); setSelectedUser(null); };
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
      await completeMediaUpload(requested.media_uuid);
      const avatar = await updateMyAvatar(requested.media_uuid);
      setProfile((current) => current ? { ...current, ...avatar } : current);
      setNotice({ status: 'success', title: '头像已更新', description: '新头像已经保存。' });
    } catch (error) {
      if (error.status === 401) return onSignedOut(error.message || '登录状态已失效，请重新登录。');
      setNotice({ status: 'danger', title: '头像更新失败', description: error.message || '请稍后重试。' });
    } finally {
      setAvatarUploading(false);
    }
  };

  const updateSearchFilter = (field, value) => {
    setSearchFilters((current) => ({ ...current, [field]: value }));
    setSearchErrors((current) => ({ ...current, [field]: undefined }));
  };
  const updateSearchCountry = (country) => {
    setSearchFilters((current) => ({ ...current, country, province: '' }));
    setSearchErrors((current) => ({ ...current, country: undefined, province: undefined }));
  };

  const changeSearchMode = (mode) => {
    setSearchMode(mode);
    setSearchErrors({});
    setSearchResults([]);
    setSearchMeta({ next_cursor: null, has_more: false });
    setLastSearch(null);
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
      if (error.status === 401) return onSignedOut(error.message || '登录状态已失效，请重新登录。');
      const fields = { INVALID_NICKNAME: 'nickname', INVALID_GENDER: 'gender', INVALID_SIGNATURE: 'signature', INVALID_BIRTHDAY: 'birthday', INVALID_COUNTRY: 'country', INVALID_PROVINCE: 'province' };
      if (fields[error.code]) setErrors((current) => ({ ...current, [fields[error.code]]: error.message }));
      setNotice({ status: 'danger', title: '保存失败', description: error.message });
    } finally { setSaving(false); }
  };

  const handleSearch = async (event, append = false) => {
    event?.preventDefault();
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
    if (Object.keys(nextErrors).length) return;

    if (!append) {
      setSearching(true);
      setSelectedUser(null);
      setSearchResults([]);
      setSearchMeta({ next_cursor: null, has_more: false });
      setLastSearch(nextFilters);
    }

    try {
      const result = await searchUsers(nextFilters);
      setSearchResults((current) => append ? [...current, ...result.users] : result.users);
      setSearchMeta(result.meta);
    } catch (error) {
      if (error.status === 401) return onSignedOut(error.message || '登录状态已失效，请重新登录。');
      const fields = { INVALID_PHONE: 'phone', INVALID_NICKNAME: 'nickname', INVALID_COUNTRY: 'country', INVALID_PROVINCE: 'province', INVALID_GENDER: 'gender' };
      if (fields[error.code]) setSearchErrors((current) => ({ ...current, [fields[error.code]]: error.message }));
      setNotice({ status: 'danger', title: '搜索失败', description: error.message });
    } finally {
      setSearching(false);
      setLoadingMore(false);
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

  const renderPanel = () => {
    if (loading) return <div className="native-panel-state"><Spinner /><p>正在加载…</p></div>;
    if (!profile) return <div className="native-panel-state"><Notice notice={notice} /><button className="native-primary-button" onClick={loadProfile}>重新加载</button></div>;

    if (view === 'chats') return <>
      <header className="context-head"><h1>消息</h1><button className="square-button" aria-label="新建会话"><Icon name="plus" /></button></header>
      <label className="context-search"><Icon name="search" size={18} /><input placeholder="搜索会话…" /></label>
      <div className="conversation-scroll"><h2>收藏</h2>{conversations.slice(0, 2).map((item, index) => <button className={`conversation-item ${index === 0 ? 'active' : ''}`} key={item.name}><DemoAvatar item={item} /><span><strong>{item.name}</strong><small>{item.preview}</small></span><time>{item.time}</time>{item.badge && <em>{String(item.badge).padStart(2, '0')}</em>}</button>)}<h2>消息</h2>{conversations.slice(2).map((item) => <button className="conversation-item" key={item.name}><DemoAvatar item={item} /><span><strong>{item.name}</strong><small>{item.preview}</small></span><time>{item.time}</time>{item.badge && <em>{String(item.badge).padStart(2, '0')}</em>}</button>)}</div>
    </>;

    if (view === 'contacts' && selectedUser) return <ContactDetail user={selectedUser} onBack={() => setSelectedUser(null)} />;

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
        {searchResults.length > 0 && <div className="contact-result-list">{searchResults.map((user, index) => <SearchResult user={user} onOpen={setSelectedUser} key={`${user.nickname}-${user.birthday || ''}-${index}`} />)}{searchMeta.has_more && <button className="load-more-users" type="button" onClick={() => handleSearch(null, true)} disabled={loadingMore}>{loadingMore ? <><Spinner />加载中</> : '加载更多'}</button>}</div>}
      </div>
    </>;

    if (view === 'profile') return <>
      <header className="context-head"><h1>个人资料</h1>{!editingProfile && <button className="icon-button" onClick={() => { setEditingProfile(true); setNotice(null); setErrors({}); }} aria-label="编辑资料"><Icon name="edit" size={18} /></button>}</header>
      <div className="reference-profile-cover" />
      <div className="reference-profile-content"><div className={`reference-avatar-wrap ${editingProfile ? 'avatar-editable' : ''}`}><UserAvatar profile={profile} size="xl" />{editingProfile ? <><input ref={avatarInputRef} className="avatar-file-input" type="file" accept="image/*" onChange={handleAvatarFile} /><button className="avatar-edit-trigger" type="button" onClick={() => avatarInputRef.current?.click()} disabled={avatarUploading} aria-label="更换头像">{avatarUploading ? <Spinner /> : <Icon name="camera" size={16} />}</button></> : <i />}</div><h2>{profile.nickname}</h2><span>在线</span>
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

    return <><header className="context-head"><h1>{view === 'calls' ? '通话' : '群组'}</h1><Icon name={view} /></header><div className="panel-empty"><Icon name={view} size={38} /><h2>功能准备中</h2><p>对应接口接入后会在这里展示内容。</p></div></>;
  };

  return <main className="reference-app">
    <aside className="reference-rail"><button className="reference-logo" onClick={() => navigate('chats')} aria-label="Mocha 首页"><Icon name="chats" size={28} /></button><nav aria-label="主要功能">{navItems.map((item) => <button key={item.id} className={view === item.id ? 'active' : ''} onClick={() => navigate(item.id)}><Icon name={item.icon} /><span>{item.label}</span></button>)}</nav><div className="rail-bottom"><button onClick={handleLogout} disabled={loggingOut}>{loggingOut ? <Spinner /> : <Icon name="logout" />}<span>退出</span></button><button className="rail-user" onClick={() => navigate('profile')} aria-label="打开个人资料"><UserAvatar profile={profile || session.user} size="sm" /></button></div></aside>
    <aside className="reference-context" ref={contextRef}>{renderPanel()}</aside>
    <ConversationArea />
  </main>;
}
