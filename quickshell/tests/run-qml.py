#!/usr/bin/env python3
"""Run the shell tests.

logic  node --test over tests/*.test.mjs: pure functions lifted out of the QML
       sources, no Qt involved
shell  every tests/qml/*.qml as the shell.qml of a scratch root built from
       symlinks to the repo tree, in its own offscreen Quickshell with a
       throwaway HOME and XDG tree, no socket and no session bus

Each fixture logs `FIXTURE PASS` when every check passed. A missing marker, a
non-zero exit, a QML error line before the marker or a timeout fails the run
and prints that fixture's full log. Warnings after the marker are engine
teardown and ignored.
"""

import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time

repo = Path(__file__).resolve().parents[2]
shell_dir = repo / "quickshell"
tests_dir = shell_dir / "tests"
fixture_timeout = 10
ansi = re.compile(r"\x1b\[[0-9;]*m")
qml_error = re.compile(r"ReferenceError|TypeError|is not a type|Unable to assign|Cannot assign|is not defined|Could not load|Component is not ready|Binding loop|is not installed|Cannot read property")
host_env_prefixes = ("DFILES_", "QS_", "QT_", "XDG_", "LC_", "LANG", "TZ")
host_display_vars = ("WAYLAND_DISPLAY", "DISPLAY", "DBUS_SESSION_BUS_ADDRESS")


def quickshell():
    override = os.environ.get("QS")
    candidates = [override] if override else ["qs", "quickshell"]
    for candidate in candidates:
        executable = shutil.which(candidate)
        if executable:
            return executable
    raise SystemExit("quickshell is required; set QS to its path")


def start(command, cwd, env=None):
    try:
        return subprocess.Popen(command, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, start_new_session=True), None
    except OSError as error:
        return None, str(error)


def isolated_env(root):
    env = {key: value for key, value in os.environ.items() if not key.startswith(host_env_prefixes) and key not in host_display_vars}
    home = root / "home"
    runtime = root / "run"
    for path in (home, runtime):
        path.mkdir(parents=True)
    runtime.chmod(0o700)
    env.update({
        "HOME": str(home),
        "XDG_CONFIG_HOME": str(home / ".config"),
        "XDG_CACHE_HOME": str(home / ".cache"),
        "XDG_DATA_HOME": str(home / ".local" / "share"),
        "XDG_STATE_HOME": str(home / ".local" / "state"),
        "XDG_RUNTIME_DIR": str(runtime),
        "XDG_DATA_DIRS": str(root / "share"),
        "XDG_CONFIG_DIRS": str(root / "etc"),
        "QT_QPA_PLATFORM": "offscreen",
        "LC_ALL": "C.UTF-8",
        "TZ": "UTC",
    })
    return env


def scratch_root(root, fixture):
    harness = root / "shell"
    harness.mkdir()
    for entry in shell_dir.iterdir():
        if entry.name in ("shell.qml", "scripts", "tests", ".qmlls.ini"):
            continue
        (harness / entry.name).symlink_to(entry)
    shutil.copy(fixture, harness / "shell.qml")
    if fixture.stem == "preview":
        for name in ("preview.pdf", "preview.png"):
            shutil.copy(tests_dir / "qml" / name, harness / name)
    return harness


def communicate(process, timeout):
    try:
        output, _ = process.communicate(timeout=timeout)
        return output, False
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGTERM)
        try:
            output, _ = process.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            output, _ = process.communicate()
        return output + "\nfixture timed out", True


def run_fixture(fixture, qs):
    started = time.monotonic()
    with tempfile.TemporaryDirectory(prefix="dfiles-fixture-") as tmp:
        root = Path(tmp)
        harness = scratch_root(root, fixture)
        env = isolated_env(root)
        process, error = start([qs, "-p", str(harness)], root, env)
        if process is None:
            return fixture.stem, False, time.monotonic() - started, error
        output, timed_out = communicate(process, fixture_timeout)
    output = ansi.sub("", output)
    checks, marker, _ = output.partition("FIXTURE PASS")
    passed = not timed_out and process.returncode == 0 and marker != "" and not qml_error.search(checks)
    return fixture.stem, passed, time.monotonic() - started, output


def run_logic():
    started = time.monotonic()
    tests = sorted(str(path.relative_to(repo)) for path in tests_dir.glob("*.test.mjs"))
    if not tests:
        raise SystemExit("no *.test.mjs under quickshell/tests")
    process, error = start(["node", "--test", *tests], repo)
    if process is None:
        return "logic", False, time.monotonic() - started, error
    output, timed_out = communicate(process, fixture_timeout)
    return "logic", not timed_out and process.returncode == 0, time.monotonic() - started, output


def report(name, passed, elapsed, output):
    print(f"{'PASS' if passed else 'FAIL'} {name}: {elapsed:.2f}s", flush=True)
    if not passed:
        print(output, flush=True)
        return
    for line in output.splitlines():
        if "FAIL " in line or "# tests " in line or "# fail " in line:
            print("  " + line.strip(), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--jobs", type=int, default=max(1, min(4, len(os.sched_getaffinity(0)) // 2)))
    parser.add_argument("suites", nargs="*", choices=["logic", "shell"])
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error("--jobs must be positive")
    selected = args.suites or ["logic", "shell"]

    jobs = []
    if "logic" in selected:
        jobs.append((run_logic, ()))
    if "shell" in selected:
        qs = quickshell()
        fixtures = sorted((tests_dir / "qml").glob("*.qml"))
        if not fixtures:
            raise SystemExit("no fixtures under quickshell/tests/qml")
        jobs.extend((run_fixture, (fixture, qs)) for fixture in fixtures)

    started = time.monotonic()
    failed = False
    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        futures = [pool.submit(fn, *fn_args) for fn, fn_args in jobs]
        for future in as_completed(futures):
            name, passed, elapsed, output = future.result()
            failed = failed or not passed
            report(name, passed, elapsed, output)
    print(f"QML checks: {time.monotonic() - started:.2f}s", flush=True)
    return int(failed)


if __name__ == "__main__":
    sys.exit(main())
