#!/usr/bin/env python3
"""Prepare the private-widget test in a disposable pinned upstream checkout.

This adds a new test library only; it never edits the original Flutter widget.
Run flutter pub get with both native-asset hooks disabled in that checkout first.
"""
import pathlib
import sys

checkout = pathlib.Path(sys.argv[1]).resolve()
source = checkout / 'lib/views/dashboard/widgets/current_outbound.dart'
harness = pathlib.Path(__file__).with_name('port_dashboard_morph_test.dart').read_text()
# Dart imports must precede declarations; keep the source body byte-for-byte.
imports, body = harness.split('void main()', 1)
out = source.with_name('dashboard_morph_reference_test.dart')
if out.exists():
    raise SystemExit(f'Refusing to overwrite {out}')
out.write_text(imports + '\n' + source.read_text() + '\nvoid main()' + body)
print(out)
