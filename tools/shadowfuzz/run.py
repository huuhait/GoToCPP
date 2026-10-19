#!/usr/bin/env python3
"""usage: run.py DIR GOCPP [JOBS]

For every DIR/*.go (gen.py): runs it with real Go and with `GOCPP run` (a copy named .gp) and
reports the programs whose stdout or exit code differ. Set GO to the go command, and
GOCPP_GATE_PHASE (default 08) and XDG_CACHE_HOME in the environment. Exit 1 if any differ.
"""
import concurrent.futures
import glob
import os
import shutil
import signal
import subprocess
import sys
import tempfile


def sh(cmd, timeout, **kw):
    """Runs cmd in a process group of its own, which dies with a timeout (no orphan programs)."""
    p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True, **kw)
    try:
        out, err = p.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        os.killpg(p.pid, signal.SIGKILL)
        p.communicate()
        raise
    return subprocess.CompletedProcess(cmd, p.returncode, out, err)


def one(path, gocpp):
    d = tempfile.mkdtemp(prefix="shadowfuzz.", dir=os.environ.get("TMPDIR"))
    try:
        base = os.path.basename(path)[:-3]
        shutil.copy(path, os.path.join(d, base + ".gp"))
        env = dict(os.environ)
        env.setdefault("GOCPP_GATE_PHASE", "08")
        env["GOCPP_MODCACHE"] = "off"
        env["GOCPP_OFFLINE"] = "1"
        # (a program that does not stop is a bug of the generator: it is reported, not waited for)
        go = os.environ.get("GO", "go")
        try:
            b = sh([go, "build", "-o", os.path.join(d, "ref"), path], 600, env=env, cwd=d)
            if b.returncode != 0:
                return path, "go build failed:\n" + b.stderr
            want = sh([os.path.join(d, "ref")], 60, env=env, cwd=d)
            got = sh([gocpp, "run", base + ".gp"], 900, env=env, cwd=d)
        except subprocess.TimeoutExpired as e:
            return path, "timeout: %s" % e
        if want.stdout != got.stdout or want.returncode != got.returncode:
            return path, "stdout/exit differ (go %d, gocpp %d)\n--- go\n%s--- gocpp\n%s--- gocpp stderr\n%s" % (
                want.returncode, got.returncode, want.stdout, got.stdout, got.stderr[-1500:])
        return path, None
    finally:
        shutil.rmtree(d, ignore_errors=True)


def main():
    d, gocpp = sys.argv[1], os.path.abspath(sys.argv[2])
    jobs = int(sys.argv[3]) if len(sys.argv) > 3 else 4
    paths = sorted(glob.glob(os.path.join(d, "*.go")))
    bad = 0
    with concurrent.futures.ThreadPoolExecutor(jobs) as ex:
        for path, err in ex.map(lambda p: one(p, gocpp), paths):
            if err:
                bad += 1
                print("DIFF", path, err, flush=True)
    print("%d programs, %d differ" % (len(paths), bad))
    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main()
