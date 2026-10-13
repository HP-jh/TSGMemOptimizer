#!/usr/bin/env python3
"""Push local git repo to GitHub via Git Data API."""
import base64, json, os, subprocess, sys, urllib.request, urllib.error

OWNER, REPO = "HP-jh", "TSGMemOptimizer"
TOKEN = open(os.path.expanduser("~/.aily/workspace/.github_token")).read().strip()
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

def api(path, data=None, method=None):
    url = f"https://api.github.com/repos/{OWNER}/{REPO}" + path
    body = json.dumps(data).encode() if data is not None else None
    req = urllib.request.Request(url, data=body, method=method or ("POST" if data else "GET"))
    req.add_header("Authorization", f"token {TOKEN}")
    req.add_header("Accept", "application/vnd.github.v3+json")
    req.add_header("User-Agent", "tsgmemo-push")
    if body:
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        if e.code == 404:
            return None
        print(f"HTTP {e.code}: {body[:800]}")
        raise

def git_blob_sha(content):
    header = f"blob {len(content)}\0".encode()
    import hashlib
    return hashlib.sha1(header + content).hexdigest()

def get_files():
    os.chdir(REPO_ROOT)
    out = subprocess.check_output(["git", "ls-files"]).decode().strip().split("\n")
    files = {}
    for f in out:
        if not f:
            continue
        files[f] = open(f, "rb").read()
    return files

def push():
    try:
        ref = api("/git/refs/heads/master")
    except Exception:
        ref = None
    parent_sha = ref["object"]["sha"] if ref else None

    files = get_files()
    tree_entries = []
    for path, content in files.items():
        blob = api("/git/blobs", {"encoding": "base64", "content": base64.b64encode(content).decode()})
        tree_entries.append({"path": path, "mode": "100644", "type": "blob", "sha": blob["sha"]})

    tree_data = {"tree": tree_entries}
    if parent_sha:
        commit_info = api(f"/git/commits/{parent_sha}")
        tree_data["base_tree"] = commit_info["tree"]["sha"]
    tree = api("/git/trees", tree_data)

    local_sha = subprocess.check_output(["git", "rev-parse", "HEAD"]).decode().strip()
    msg = subprocess.check_output(["git", "log", "-1", "--pretty=%B"]).decode().strip()
    an = subprocess.check_output(["git", "log", "-1", "--pretty=%an"]).decode().strip()
    ae = subprocess.check_output(["git", "log", "-1", "--pretty=%ae"]).decode().strip()
    ad = subprocess.check_output(["git", "log", "-1", "--pretty=%aI"]).decode().strip()
    cn = subprocess.check_output(["git", "log", "-1", "--pretty=%cn"]).decode().strip()
    ce = subprocess.check_output(["git", "log", "-1", "--pretty=%ce"]).decode().strip()
    cd = subprocess.check_output(["git", "log", "-1", "--pretty=%cI"]).decode().strip()

    commit_data = {
        "message": msg,
        "tree": tree["sha"],
        "author": {"name": an, "email": ae, "date": ad},
        "committer": {"name": cn, "email": ce, "date": cd},
    }
    if parent_sha:
        commit_data["parents"] = [parent_sha]

    commit = api("/git/commits", commit_data)
    new_sha = commit["sha"]

    if parent_sha:
        api("/git/refs/heads/master", {"sha": new_sha, "force": True}, "PATCH")
    else:
        api("/git/refs", {"ref": "refs/heads/master", "sha": new_sha})
    print(f"Pushed {new_sha} (local was {local_sha})")

    main_go = open(os.path.join(REPO_ROOT, 'cmd/tsgmemo/main.go')).read()
    ver = main_go.split('const version = "')[1].split('"')[0]
    tag = "v" + ver
    try:
        api("/git/refs", {"ref": f"refs/tags/{tag}", "sha": new_sha})
        print(f"Created tag {tag}")
    except Exception as e:
        print(f"Tag creation skipped: {e}")

if __name__ == "__main__":
    push()
