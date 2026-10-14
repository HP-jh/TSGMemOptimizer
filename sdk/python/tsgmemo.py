"""TSGMemOptimizer Python SDK"""
import json
import urllib.request

class TSGMemOptimizerClient:
    def __init__(self, base_url="http://127.0.0.1:29471"):
        self.base_url = base_url.rstrip('/')

    def _get(self, path):
        with urllib.request.urlopen(self.base_url + path, timeout=10) as r:
            return json.loads(r.read())

    def _post(self, path, body=None):
        data = json.dumps(body).encode() if body else b''
        req = urllib.request.Request(self.base_url + path, data=data, headers={'Content-Type': 'application/json'}, method='POST')
        with urllib.request.urlopen(req, timeout=10) as r:
            return json.loads(r.read())

    def memory(self):
        return self._get("/api/memory")

    def history(self):
        return self._get("/api/memory/history")

    def processes(self):
        return self._get("/api/processes")

    def clean(self, level=""):
        path = "/api/clean" + (f"?level={level}" if level else "")
        return self._post(path)

    def kill(self, pid):
        return self._post(f"/api/kill?pid={pid}")

    def profiles(self):
        return self._get("/api/profiles")

    def set_profile(self, name):
        return self._post(f"/api/profile?name={name}")

    def guardrails(self):
        return self._get("/api/guardrails")

    def health(self):
        return self._get("/api/health")

    def ai_schedule(self, agent_id, intent, risk_declared, reason, **kwargs):
        payload = {"agent_id": agent_id, "intent": intent, "risk_declared": risk_declared, "reason": reason}
        payload.update(kwargs)
        return self._post("/api/ai-schedule", payload)

    def openai_function_call(self, args_json):
        return self.ai_schedule(protocol="openai", args=args_json)
