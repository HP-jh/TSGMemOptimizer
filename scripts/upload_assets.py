#!/usr/bin/env python3
import json, os, urllib.request, urllib.error

OWNER, REPO = "HP-jh", "TSGMemOptimizer"
TOKEN = open(os.path.expanduser("~/.aily/workspace/.github_token")).read().strip()
VERSION = os.sys.argv[1] if len(os.sys.argv) > 1 else "1.1.0"
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

# find release by tag
releases = api("/releases")
rel = None
for r in releases:
    if r["tag_name"] == f"v{VERSION}":
        rel = r
        break
if not rel:
    print(f"Release v{VERSION} not found")
    exit(1)

upload_url = rel["upload_url"].replace("{?name,label}", "")
for f in os.listdir(DIST):
    if not f.startswith(f"tsgmemo-v{VERSION}"):
        continue
    path = os.path.join(DIST, f)
    if not os.path.isfile(path):
        continue
    data = open(path, "rb").read()
    req = urllib.request.Request(f"{upload_url}?name={f}", data=data, method="POST")
    req.add_header("Authorization", f"token {TOKEN}")
    req.add_header("Content-Type", "application/octet-stream")
    req.add_header("Accept", "application/vnd.github.v3+json")
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            print(f"Uploaded {f}")
    except urllib.error.HTTPError as e:
        if e.code == 422:
            print(f"Skipped {f} (already exists)")
        else:
            print(f"Failed {f}: {e.code}")
print(f"Done for v{VERSION}")
