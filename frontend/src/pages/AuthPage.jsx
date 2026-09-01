import React, { useEffect, useRef, useState } from 'react';
import {
  loginAccount,
  persistAuthSession,
  registerAccount,
  sendRegisterCode,
} from '../api/auth';

const phonePattern = /^1[3-9]\d{9}$/;

function Icon({ name, size = 20 }) {
  const paths = {
    chats: <><path d="M5 18.5 3.5 21v-5.2A8 8 0 1 1 7 19.5Z" /><path d="M8 10h8M8 14h5" /></>,
    phone: <><rect x="7" y="2.5" width="10" height="19" rx="2.2" /><path d="M10.5 18.5h3" /></>,
    lock: <><rect x="4" y="10" width="16" height="11" rx="2.5" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></>,
    user: <><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>,
    shield: <><path d="M12 3 20 6v5c0 5-3.3 8.4-8 10-4.7-1.6-8-5-8-10V6z" /><path d="m9 12 2 2 4-5" /></>,
    arrow: <><path d="M5 12h14" /><path d="m14 7 5 5-5 5" /></>,
    check: <path d="m5 12 4 4L19 6" />,
    info: <><circle cx="12" cy="12" r="9" /><path d="M12 11v6M12 7.5h.01" /></>,
  };

  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {paths[name] || paths.info}
    </svg>
  );
}

function AuthField({ id, label, icon, value, onChange, type = 'text', placeholder, error, autoComplete }) {
  const errorId = `${id}-error`;
  return (
    <div className={`auth-field${error ? ' invalid' : ''}`}>
      <label htmlFor={id}>{label}</label>
      <div className="auth-input-wrap">
        <span className="auth-input-icon"><Icon name={icon} size={18} /></span>
        <input
          id={id}
          type={type}
          value={value}
          placeholder={placeholder}
          autoComplete={autoComplete}
          aria-invalid={Boolean(error)}
          aria-describedby={error ? errorId : undefined}
          onChange={(event) => onChange(event.target.value)}
        />
      </div>
      {error && <p id={errorId} className="auth-field-error" role="alert">{error}</p>}
    </div>
  );
}

function Spinner() {
  return <span className="auth-spinner" aria-hidden="true" />;
}

function StatusAlert({ notice }) {
  if (!notice) return null;
  return (
    <div className={`auth-alert ${notice.status || 'accent'}`} role={notice.status === 'danger' ? 'alert' : 'status'}>
      <span className="auth-alert-icon"><Icon name={notice.status === 'success' ? 'check' : 'info'} size={18} /></span>
      <span className="auth-alert-copy">
        <strong>{notice.title}</strong>
        <span>{notice.description}</span>
      </span>
    </div>
  );
}

function OtpInput({ value, onChange, error }) {
  const inputRefs = useRef([]);
  const slots = Array.from({ length: 6 }, (_, index) => value[index] || '');

  const focusSlot = (index) => inputRefs.current[Math.max(0, Math.min(5, index))]?.focus();

  const writeDigits = (index, rawValue) => {
    const digits = rawValue.replace(/\D/g, '');
    if (!digits) {
      onChange(`${value.slice(0, index)}${value.slice(index + 1)}`);
      return;
    }

    const nextValue = `${value.slice(0, index)}${digits}${value.slice(index + digits.length)}`.slice(0, 6);
    onChange(nextValue);
    focusSlot(Math.min(5, index + digits.length));
  };

  const handleKeyDown = (event, index) => {
    if (event.key === 'Backspace' && !slots[index] && index > 0) {
      event.preventDefault();
      onChange(`${value.slice(0, index - 1)}${value.slice(index)}`);
      focusSlot(index - 1);
    } else if (event.key === 'ArrowLeft') {
      event.preventDefault();
      focusSlot(index - 1);
    } else if (event.key === 'ArrowRight') {
      event.preventDefault();
      focusSlot(index + 1);
    }
  };

  const handlePaste = (event, index) => {
    const digits = event.clipboardData.getData('text').replace(/\D/g, '');
    if (!digits) return;
    event.preventDefault();
    writeDigits(index, digits);
  };

  return (
    <div className={`auth-otp${error ? ' invalid' : ''}`} role="group" aria-label="6 位短信验证码">
      {slots.map((digit, index) => (
        <input
          key={index}
          ref={(element) => { inputRefs.current[index] = element; }}
          type="text"
          inputMode="numeric"
          pattern="[0-9]*"
          maxLength={1}
          value={digit}
          aria-label={`验证码第 ${index + 1} 位`}
          aria-invalid={Boolean(error)}
          autoComplete={index === 0 ? 'one-time-code' : 'off'}
          onChange={(event) => writeDigits(index, event.target.value)}
          onKeyDown={(event) => handleKeyDown(event, index)}
          onPaste={(event) => handlePaste(event, index)}
          onFocus={(event) => event.target.select()}
        />
      ))}
    </div>
  );
}

export default function AuthPage({ initialNotice = null, onAuthenticated }) {
  const [tab, setTab] = useState('login');
  const [phone, setPhone] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [code, setCode] = useState('');
  const [remember, setRemember] = useState(true);
  const [countdown, setCountdown] = useState(0);
  const [sending, setSending] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState({});
  const [notice, setNotice] = useState(initialNotice);

  useEffect(() => {
    if (!countdown) return undefined;
    const timer = window.setInterval(() => setCountdown((value) => Math.max(0, value - 1)), 1000);
    return () => window.clearInterval(timer);
  }, [countdown]);

  const switchTab = (key) => {
    setTab(String(key));
    setErrors({});
    setNotice(null);
  };

  const clearFieldError = (field) => {
    setErrors((current) => {
      if (!current[field]) return current;
      return { ...current, [field]: undefined };
    });
  };

  const updateField = (field, setter) => (value) => {
    setter(value);
    clearFieldError(field);
  };

  const validateLogin = () => {
    const nextErrors = {};
    if (!phonePattern.test(phone.trim())) nextErrors.phone = '请输入正确的 11 位大陆手机号';
    if (!password || new TextEncoder().encode(password).length > 64) nextErrors.password = '请输入正确的密码';
    setErrors(nextErrors);
    return Object.keys(nextErrors).length === 0;
  };

  const validateRegister = () => {
    const nextErrors = {};
    const nickname = name.trim().replace(/\s+/g, ' ');
    if (!phonePattern.test(phone.trim())) nextErrors.phone = '请输入正确的 11 位大陆手机号';
    if (Array.from(password).length < 8 || new TextEncoder().encode(password).length > 64) {
      nextErrors.password = '密码至少 8 个字符，且不能超过 64 字节';
    }
    if (!nickname || Array.from(nickname).length > 50) nextErrors.name = '昵称需为 1–50 个字符';
    if (!/^\d{6}$/.test(code)) nextErrors.code = '请输入 6 位数字验证码';
    setErrors(nextErrors);
    return Object.keys(nextErrors).length === 0;
  };

  const applyApiFieldError = (error) => {
    const fieldByCode = {
      INVALID_PHONE: 'phone',
      WEAK_PASSWORD: 'password',
      INVALID_NICKNAME: 'name',
      PHONE_REGISTERED: 'phone',
      REGISTER_CODE_INVALID: 'code',
      REGISTER_CODE_EXPIRED: 'code',
      INVALID_CREDENTIALS: 'password',
    };
    const field = fieldByCode[error.code];
    if (field) setErrors((current) => ({ ...current, [field]: error.message }));
  };

  const handleLogin = async (event) => {
    event.preventDefault();
    if (!validateLogin()) return;

    setSubmitting(true);
    setNotice(null);
    try {
      const session = await loginAccount({ phone: phone.trim(), password });
      const storedSession = persistAuthSession(session, remember);
      setErrors({});
      onAuthenticated?.(storedSession);
    } catch (requestError) {
      applyApiFieldError(requestError);
      setNotice({ status: 'danger', title: '登录失败', description: requestError.message });
    } finally {
      setSubmitting(false);
    }
  };

  const handleSendCode = async () => {
    if (!phonePattern.test(phone.trim())) {
      setErrors((value) => ({ ...value, phone: '请先输入正确的手机号' }));
      return;
    }

    setErrors((value) => ({ ...value, phone: undefined }));
    setNotice(null);
    setSending(true);
    try {
      await sendRegisterCode(phone.trim());
      setCountdown(60);
      setNotice({
        status: 'success',
        title: '验证码已发送',
        description: '验证码 5 分钟内有效，60 秒后可以重新获取。',
      });
    } catch (requestError) {
      applyApiFieldError(requestError);
      setNotice({ status: 'danger', title: '验证码发送失败', description: requestError.message });
    } finally {
      setSending(false);
    }
  };

  const handleRegister = async (event) => {
    event.preventDefault();
    if (!validateRegister()) return;

    setSubmitting(true);
    setNotice(null);
    try {
      await registerAccount({
        phone: phone.trim(),
        password,
        nickname: name.trim().replace(/\s+/g, ' '),
        code,
      });
      setCode('');
      setName('');
      setPassword('');
      setErrors({});
      setTab('login');
      setNotice({
        status: 'success',
        title: '账号创建成功',
        description: '你的手机号已保留，请输入刚才设置的密码登录。',
      });
    } catch (requestError) {
      applyApiFieldError(requestError);
      setNotice({ status: 'danger', title: '注册失败', description: requestError.message });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="auth-page">
      <section className="auth-showcase" aria-label="Mocha 产品介绍">
        <div className="showcase-glow showcase-glow-one" />
        <div className="showcase-glow showcase-glow-two" />
        <div className="brand"><span className="brand-mark"><Icon name="chats" size={22} /></span><b>Mocha</b><span>团队沟通</span></div>
        <div className="showcase-copy">
          <span className="showcase-kicker">FOCUS · CONNECT · CREATE</span>
          <h1>沟通应该轻一点，<br />进展应该快一点。</h1>
          <p>一个安静、清晰的团队空间，让每条重要消息都被看见。</p>
        </div>
        <div className="message-preview">
          <div className="message-avatar warm">夏</div>
          <div><small>林夏 · 刚刚</small><p>新版交互稿已经同步，下午一起走查 ✨</p></div>
        </div>
        <div className="showcase-foot"><Icon name="shield" size={17} /> 安全连接 · 专注协作</div>
      </section>

      <section className="auth-content">
        <div className="mobile-brand brand"><span className="brand-mark"><Icon name="chats" size={21} /></span><b>Mocha</b></div>
        <div className="auth-card">
          <header className="auth-card-header">
            <h1>欢迎使用 Mocha</h1>
            <p>登录或创建账号，继续你的团队协作</p>
          </header>
          <div className="auth-card-content">
            <div className="auth-tabs">
              <div className="auth-tab-list" role="tablist" aria-label="登录与注册">
                <button id="login-tab" type="button" role="tab" aria-selected={tab === 'login'} aria-controls="login-panel" tabIndex={tab === 'login' ? 0 : -1} onClick={() => switchTab('login')}>登录</button>
                <button id="register-tab" type="button" role="tab" aria-selected={tab === 'register'} aria-controls="register-panel" tabIndex={tab === 'register' ? 0 : -1} onClick={() => switchTab('register')}>注册</button>
              </div>

              {tab === 'login' && (
                <section id="login-panel" className="auth-tab-panel" role="tabpanel" aria-labelledby="login-tab">
                  <form className="auth-form" onSubmit={handleLogin} noValidate>
                    <AuthField id="login-phone" label="手机号" icon="phone" value={phone} onChange={updateField('phone', setPhone)} placeholder="请输入手机号" error={errors.phone} autoComplete="tel" />
                    <AuthField id="login-password" label="密码" icon="lock" value={password} onChange={updateField('password', setPassword)} type="password" placeholder="请输入密码" error={errors.password} autoComplete="current-password" />
                    <div className="login-options">
                      <label className="auth-checkbox">
                        <input type="checkbox" checked={remember} onChange={(event) => setRemember(event.target.checked)} />
                        <span className="auth-checkbox-control"><Icon name="check" size={14} /></span>
                        <span>保持登录</span>
                      </label>
                      <button className="auth-link-button" type="button" onClick={() => setNotice({ status: 'accent', title: '重置密码', description: '请联系管理员重置密码。' })}>忘记密码？</button>
                    </div>
                    <button className="auth-submit" type="submit" disabled={submitting}>
                      {submitting ? <><Spinner />正在验证</> : <>登录 Mocha <Icon name="arrow" size={18} /></>}
                    </button>
                  </form>
                </section>
              )}

              {tab === 'register' && (
                <section id="register-panel" className="auth-tab-panel" role="tabpanel" aria-labelledby="register-tab">
                  <form className="auth-form" onSubmit={handleRegister} noValidate>
                    <AuthField id="register-phone" label="手机号" icon="phone" value={phone} onChange={updateField('phone', setPhone)} placeholder="请输入手机号" error={errors.phone} autoComplete="tel" />
                    <div className="code-heading"><label>短信验证码</label><button className="auth-code-button" type="button" disabled={sending || countdown > 0} onClick={handleSendCode}>{sending ? <><Spinner />发送中</> : countdown ? `${countdown}s 后重发` : '获取验证码'}</button></div>
                    <OtpInput value={code} onChange={updateField('code', setCode)} error={errors.code} />
                    {errors.code && <p className="standalone-error" role="alert">{errors.code}</p>}
                    <AuthField id="register-name" label="你的名字" icon="user" value={name} onChange={updateField('name', setName)} placeholder="方便团队伙伴找到你" error={errors.name} autoComplete="name" />
                    <AuthField id="register-password" label="设置密码" icon="lock" value={password} onChange={updateField('password', setPassword)} type="password" placeholder="至少 8 位" error={errors.password} autoComplete="new-password" />
                    <button className="auth-submit" type="submit" disabled={submitting}>
                      {submitting ? <><Spinner />正在提交</> : <>创建账号 <Icon name="arrow" size={18} /></>}
                    </button>
                    <p className="agreement">注册即表示你同意服务条款与隐私政策</p>
                  </form>
                </section>
              )}
            </div>

            <StatusAlert notice={notice} />
            {!notice && <StatusAlert notice={{ status: 'accent', title: '账号安全', description: '验证码 5 分钟内有效；请勿将验证码或密码告诉他人。' }} />}
          </div>
          <footer className="auth-card-footer"><span>© 2026 Mocha</span><span>安全 · 简单 · 专注</span></footer>
        </div>
      </section>
    </main>
  );
}
