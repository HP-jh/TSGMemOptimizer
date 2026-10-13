#!/usr/bin/env python3
import json, os, urllib.request

OWNER, REPO = "HP-jh", "TSGMemOptimizer"
TOKEN = open(os.path.expanduser("~/.aily/workspace/.github_token")).read().strip()
VERSION = os.sys.argv[1] if len(os.sys.argv) > 1 else "1.0.0"
DIST = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "dist")

def api(path, data=None, method=None):
    url = f"https://api.github.com/repos/{OWNER}/{REPO}" + path
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(url, data=body, method=method or ("POST" if data else "GET"))
    req.add_header("Authorization", f"token {TOKEN}")
    req.add_header("Accept", "application/vnd.github.v3+json")
    if body:
        req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read().decode())

# create release
release = api("/releases", {
    "tag_name": f"v{VERSION}",
    "name": f"TSGMemOptimizer v{VERSION}",
    "body": f"Release v{VERSION}",
    "draft": False,
    "prerelease": False,
})
upload_url = release["upload_url"].replace("{?name,label}", "")

# upload assets
for f in os.listdir(DIST):
    path = os.path.join(DIST, f)
    if os.path.isfile(path):
        data = open(path, "rb").read()
        req = urllib.request.Request(f"{upload_url}?name={f}", data=data, method="POST")
        req.add_header("Authorization", f"token {TOKEN}")
        req.add_header("Content-Type", "application/octet-stream")
        req.add_header("Accept", "application/vnd.github.v3+json")
        with urllib.request.urlopen(req, timeout=120) as resp:
            print(f"Uploaded {f}")
print(f"Release created: {release['html_url']}")
