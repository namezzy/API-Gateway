import axios from 'axios';

export const api = axios.create({});

// Request interceptor attaches Authorization header if token stored
api.interceptors.request.use(cfg => {
  const raw = localStorage.getItem('gateway.tokens');
  if (raw) {
    try {
      const { access } = JSON.parse(raw);
      if (access) cfg.headers = { ...cfg.headers, Authorization: 'Bearer ' + access };
    } catch {}
  }
  return cfg;
});

// Response interceptor global error handling (401 handled by caller)
api.interceptors.response.use(r => r, err => {
  return Promise.reject(err);
});
