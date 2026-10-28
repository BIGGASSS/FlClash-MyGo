#!/usr/bin/env python3
"""Owned validation only: no source writes, existing display, proxy or TUN changes.

Linux amd64 host: python3 tool/native/validate.py [--phase all|matrix|checks|smoke]
Requires Go, bash and (for smoke) Xvfb, xwininfo, ImageMagick and Tesseract.
Logs and artifacts stay in ignored build/. Re-run after concurrent edits settle.
Exit 0 = passed/stable, 1 = command failure, 2 = passed but checkout changed.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
import shutil
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / 'build/validation-native'
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--phase', choices=('all', 'matrix', 'checks', 'smoke'), default='all')
parser.add_argument('--xvfb', help='Xvfb executable (default: extracted local tooling, then PATH)')
args = parser.parse_args()
RUN = BASE / datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S.%fZ')
RUN.mkdir(parents=True, mode=0o700)
(BASE / 'latest-run.txt').write_text(str(RUN) + '\n')
env = os.environ.copy()
# Never let tests or the Xvfb launcher connect to the workstation's display.
env.pop('DISPLAY', None)
env.pop('WAYLAND_DISPLAY', None)
temp_dir = ROOT / 'build/tmp'
temp_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
env.update(CGO_ENABLED='0', GOMAXPROCS='4', TMPDIR=str(temp_dir), GOTMPDIR=str(temp_dir))
env['GOFLAGS'] = (env.get('GOFLAGS', '') + ' -mod=readonly -p=4').strip()
for name in ('GOOS', 'GOARCH', 'OUT_DIR', 'FLCLASH_TEST_CORE', 'FLCLASH_SNAPSHOT_DIR'):
    env.pop(name, None)
results = []

def fingerprint():
    paths = subprocess.check_output(['git', 'ls-files', '-co', '--exclude-standard', '-z'], cwd=ROOT).split(b'\0')
    hashes = {}
    for raw in set(paths):
        if not raw:
            continue
        name = os.fsdecode(raw)
        path = ROOT / name
        if path.is_file():
            try:
                hashes[name] = hashlib.sha256(path.read_bytes()).hexdigest()
            except FileNotFoundError:
                hashes[name] = 'removed-during-fingerprint'
    return hashes

initial = fingerprint()
(RUN / 'sources.before.json').write_text(json.dumps(initial, indent=2, sort_keys=True) + '\n')

def run(name, command, cwd=ROOT, overrides=None, timeout=2400):
    command_env = dict(env)
    command_env.update(overrides or {})
    start = datetime.now(timezone.utc)
    before = fingerprint()
    print(f'{start.isoformat()} START {name}: {" ".join(map(str, command))}', flush=True)
    with (RUN / (name + '.log')).open('w') as log:
        log.write(f'cwd={cwd}\ncommand={command!r}\nCGO_ENABLED=0 GOMAXPROCS=4 GOFLAGS={command_env["GOFLAGS"]}\n')
        log.flush()
        try:
            completed = subprocess.run(command, cwd=cwd, env=command_env, stdout=log, stderr=subprocess.STDOUT, timeout=timeout)
            code = completed.returncode
        except subprocess.TimeoutExpired:
            code = 124
            log.write('\nVALIDATION COMMAND TIMED OUT\n')
        except OSError as error:
            code = 127
            log.write(f'\n{error}\n')
    after = fingerprint()
    changed = sorted(name for name in before.keys() | after.keys() if before.get(name) != after.get(name))
    item = {'name': name, 'command': list(map(str, command)), 'cwd': str(cwd), 'exit_code': code,
            'started': start.isoformat(), 'ended': datetime.now(timezone.utc).isoformat(), 'source_changes_during_command': changed}
    results.append(item)
    (RUN / 'results.json').write_text(json.dumps(results, indent=2) + '\n')
    print(f'{item["ended"]} END {name}: exit={code}, source_changes={len(changed)}', flush=True)
    return code

if args.phase in ('all', 'matrix'):
    for goos, goarch in (('linux', 'amd64'), ('linux', 'arm64'), ('windows', 'amd64'), ('windows', 'arm64'), ('darwin', 'amd64'), ('darwin', 'arm64')):
        run(f'build-{goos}-{goarch}', ['bash', 'scripts/build.sh'], overrides={'GOOS': goos, 'GOARCH': goarch})

if args.phase in ('all', 'checks'):
    core = ROOT / 'build/linux-amd64/flclash-core'
    core_env = {'FLCLASH_TEST_CORE': str(core)}
    if core.is_file():
        run('real-core-ipc', ['go', 'test', './internal/coreclient', './internal/app', '-run', 'TestRealCore|TestEngineLatestIntentAndFailedConfig|TestServiceOutboundRealCoreLoopbackCache|TestOnDemandRealCore', '-count=1', '-v'], overrides=core_env)
    else:
        print('BLOCKED real-core-ipc: Linux amd64 core not built', flush=True)
        results.append({'name': 'real-core-ipc', 'exit_code': 127, 'blocked': str(core)})
    run('root-tests', ['go', 'test', './...', '-count=1'], overrides=core_env if core.is_file() else {})
    run('root-vet', ['go', 'vet', './...'])
    run('core-tests', ['go', 'test', './...', '-count=1'], cwd=ROOT / 'core')
    run('core-vet', ['go', 'vet', './...'], cwd=ROOT / 'core')
    run('core-tests-with-gvisor', ['go', 'test', '-tags', 'with_gvisor', './...', '-count=1'], cwd=ROOT / 'core')
    run('core-vet-with-gvisor', ['go', 'vet', '-tags', 'with_gvisor', './...'], cwd=ROOT / 'core')

if args.phase in ('all', 'smoke'):
    tooling = ROOT / 'build/tooling/native-smoke/root/usr/bin'
    bundled_xvfb = tooling / 'Xvfb'
    xvfb = args.xvfb or (str(bundled_xvfb) if bundled_xvfb.is_file() else shutil.which('Xvfb') or 'Xvfb')
    runtime_dir = RUN / 'runtime'
    runtime_dir.mkdir(mode=0o700)
    data_dir = RUN / 'data'
    data_dir.mkdir()
    smoke_env = {'PATH': str(tooling) + os.pathsep + env['PATH'],
                 'XDG_RUNTIME_DIR': str(runtime_dir), 'XDG_DATA_HOME': str(data_dir)}
    for renderer, flags in (('default', []), ('cpu', ['--cpu'])):
        run('native-smoke-' + renderer, [sys.executable, str(ROOT / 'tool/native/smoke.py'), '--binary', str(ROOT / 'build/linux-amd64/flclash'), '--xvfb', xvfb, '--output', str(RUN / ('native-smoke-' + renderer)), *flags], overrides=smoke_env, timeout=900)

final = fingerprint()
changed = sorted(name for name in initial.keys() | final.keys() if initial.get(name) != final.get(name))
(RUN / 'sources.after.json').write_text(json.dumps(final, indent=2, sort_keys=True) + '\n')
summary = {'phase': args.phase, 'run': str(RUN), 'results': results, 'source_changes_during_run': changed,
           'note': 'A changing source snapshot requires final retest after concurrent edits settle; cross-builds do not validate OS runtime behavior.'}
(RUN / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
print(json.dumps({'summary': str(RUN / 'summary.json'), 'failed': [r['name'] for r in results if r['exit_code']], 'source_changes': changed}), flush=True)
sys.exit(1 if any(r['exit_code'] for r in results) else (2 if changed else 0))
