import React, { useState } from 'react';
import { clearAuthSession, readAuthSession } from './api/auth';
import AuthPage from './pages/AuthPage';
import WorkspacePage from './pages/WorkspacePage';

export default function App() {
  const [session, setSession] = useState(() => readAuthSession());
  const [authNotice, setAuthNotice] = useState(null);

  const handleAuthenticated = (nextSession) => {
    setAuthNotice(null);
    setSession(nextSession);
  };

  const handleSignedOut = (message) => {
    clearAuthSession();
    setSession(null);
    setAuthNotice(message ? {
      status: 'warning',
      title: '需要重新登录',
      description: message,
    } : null);
  };

  if (session) {
    return <WorkspacePage session={session} onSignedOut={handleSignedOut} />;
  }

  return <AuthPage initialNotice={authNotice} onAuthenticated={handleAuthenticated} />;
}
