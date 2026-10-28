import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:fl_clash/ui/color.dart';
import 'package:fl_clash/ui/surface.dart';

void main() {
  test('export pinned FlClash theme roles', () async {
    final rows = <String, Object?>{};
    for (final seed in [
      0xff007aff,
      0xffcb30e0,
      0xffff2d55,
      0xffff383c,
      0xffff8d28,
      0xffffcc00,
      0xff34c759,
      0xff8e8e93,
      0xff663399,
      0xff13579b,
    ]) {
      for (final style in ColorStyle.values) {
        for (final dark in [false, true]) {
          for (final black in [false, true]) {
            final s = seededColorScheme(
              seedColor: Color(seed),
              brightness: dark ? Brightness.dark : Brightness.light,
              style: style,
            ).toPureBlack(black);
            String hex(Color c) =>
                '#${(c.toARGB32() & 0xffffff).toRadixString(16).padLeft(6, '0')}';
            final colors = {
              'page': SurfaceLevel.page.colorOf(s),
              'card': SurfaceLevel.card.colorOf(s),
              'block': SurfaceLevel.block.colorOf(s),
              'inset': SurfaceLevel.inset.colorOf(s),
              'chrome': s.chromeSurface,
              'text': s.onSurface,
              'muted': s.onSurfaceVariant,
              'accent': s.primary,
              'onAccent': s.onPrimary,
              'secondaryContainer': s.secondaryContainer,
              'onSecondaryContainer': s.onSecondaryContainer,
              'success': s.success,
              'warning': s.warning,
              'danger': s.error,
              'onDanger': s.onError,
              'line': s.outlineVariant,
              'outline': s.outline,
              'successContainer': s.successContainer,
              'onSuccessContainer': s.onSuccessContainer,
              'warningContainer': s.warningContainer,
              'onWarningContainer': s.onWarningContainer,
              'dangerContainer': s.errorContainer,
              'onDangerContainer': s.onErrorContainer,
            };
            rows['${hex(Color(seed))}/${style.name}/$dark/$black'] = {
              for (final e in colors.entries) e.key: hex(e.value),
            };
          }
        }
      }
    }
    final path = Platform.environment['FLCLASH_PALETTE_PATH']!;
    await File(path).parent.create(recursive: true);
    await File(path)
        .writeAsString(const JsonEncoder.withIndent('  ').convert(rows));
  });
}
