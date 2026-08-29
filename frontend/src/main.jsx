import React from 'react';
import { createRoot } from 'react-dom/client';
import '@heroui/react/styles';
import './styles.css';
import './workspace.css';
import App from './App';

createRoot(document.getElementById('root')).render(<App />);
