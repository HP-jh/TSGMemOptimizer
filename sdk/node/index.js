const http = require('http');

class TSGMemOptimizerClient {
  constructor(baseURL = 'http://127.0.0.1:29471') {
    this.baseURL = baseURL.replace(/\/$/, '');
  }

  _req(path, method = 'GET', body = null) {
    return new Promise((resolve, reject) => {
      const url = new URL(path, this.baseURL);
      const opts = { hostname: url.hostname, port: url.port, path: url.pathname + url.search, method, headers: {} };
      if (body) {
        opts.headers['Content-Type'] = 'application/json';
      }
      const req = http.request(opts, (res) => {
        let data = '';
        res.on('data', chunk => data += chunk);
        res.on('end', () => {
          try { resolve(JSON.parse(data)); } catch { resolve(data); }
        });
      });
      req.on('error', reject);
      req.setTimeout(10000);
      if (body) req.write(JSON.stringify(body));
      req.end();
    });
  }

  memory() { return this._req('/api/memory'); }
  history() { return this._req('/api/memory/history'); }
  processes() { return this._req('/api/processes'); }
  clean(level = '') { return this._req('/api/clean' + (level ? `?level=${level}` : ''), 'POST'); }
  kill(pid) { return this._req(`/api/kill?pid=${pid}`, 'POST'); }
  profiles() { return this._req('/api/profiles'); }
  setProfile(name) { return this._req(`/api/profile?name=${name}`, 'POST'); }
  guardrails() { return this._req('/api/guardrails'); }
  health() { return this._req('/api/health'); }
  aiSchedule(payload) { return this._req('/api/ai-schedule', 'POST', payload); }
}

module.exports = { TSGMemOptimizerClient };
