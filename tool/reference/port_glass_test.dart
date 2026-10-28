// Run in a throwaway checkout of the pinned FlClash source, with both build
// hooks disabled. Widget tests may not support shader filters; export that fact.
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:fl_clash/models/config.dart';
import 'package:fl_clash/ui/color.dart';
import 'package:fl_clash/ui/icons/icons.dart';
import 'package:fl_clash/ui/liquid_glass.dart';
import 'package:fl_clash/ui/navigation_dock.dart';
import 'package:fl_clash/ui/shape.dart';
import 'package:fl_clash/ui/typography.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_color_utilities/material_color_utilities.dart'
    show Hct;
import 'package:material_ui/material_ui.dart';

const bands = [
  Color(0xffffd5cf),
  Color(0xffcfe5ff),
  Color(0xffd6f4d3),
  Color(0xffffe8b7),
  Color(0xffe4d4ff),
];

class GlassReferenceBinding extends AutomatedTestWidgetsFlutterBinding {
  @override
  bool get disableShadows => false;
}

void main() {
  GlassReferenceBinding();
  final directory = Platform.environment['FLCLASH_REFERENCE_DIR']!;
  setUpAll(() async {
    await Directory(directory).create(recursive: true);
    final sdk = Platform.environment['FLUTTER_ROOT']!;
    for (final file in [
      'Roboto-Regular.ttf',
      'Roboto-Medium.ttf',
      'Roboto-Bold.ttf',
    ]) {
      final bytes = await File('$sdk/bin/cache/artifacts/material_fonts/$file')
          .readAsBytes();
      await (FontLoader(
        'Roboto',
      )..addFont(Future.value(ByteData.sublistView(bytes)))).load();
    }
  });
  test('export original glass colors and preferences', () async {
    String rgba(Color value) {
      final argb = value.toARGB32();
      return '#${(argb & 0xffffff).toRadixString(16).padLeft(6, '0')}${(argb >> 24).toRadixString(16).padLeft(2, '0')}';
    }

    final rows = <String, Object>{};
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
            final scheme = seededColorScheme(
              seedColor: Color(seed),
              brightness: dark ? Brightness.dark : Brightness.light,
              style: style,
            ).toPureBlack(black);
            final hct = Hct.fromInt(scheme.secondary.toARGB32())
              ..tone = dark ? 4 : 90;
            final key =
                '#${(seed & 0xffffff).toRadixString(16).padLeft(6, '0')}/${style.name}/$dark/$black';
            rows[key] = {
              'regularTint': rgba(LiquidGlassStyle.regular(scheme).tint),
              'pillTint': rgba(LiquidGlassStyle.pill(scheme).tint),
              'platter': rgba(Color(hct.toInt()).withValues(alpha: 0.85)),
            };
          }
        }
      }
    }
    await File('$directory/glass-colors.json').writeAsString(
      const JsonEncoder.withIndent('  ').convert({
        'default': ThemeProps.fromJson({}).liquidGlass.name,
        'legacyTrue': ThemeProps.fromJson({'liquidGlass': true})
            .liquidGlass
            .name,
        'legacyFalse': ThemeProps.fromJson({'liquidGlass': false})
            .liquidGlass
            .name,
        'shaderFilters': ui.ImageFilter.isShaderFilterSupported,
        'colors': rows,
      }),
    );
  });
  for (final dark in [false, true]) {
    for (final held in [false, true]) {
      testWidgets(
        'glass dock ${dark ? 'dark' : 'light'} ${held ? 'held' : 'rest'}',
        (tester) async {
          tester.view.physicalSize = const Size(380, 400);
          tester.view.devicePixelRatio = 1;
          addTearDown(tester.view.resetPhysicalSize);
          addTearDown(tester.view.resetDevicePixelRatio);
          final scheme = seededColorScheme(
            seedColor: const Color(0xff007aff),
            brightness: dark ? Brightness.dark : Brightness.light,
          );
          final theme = ThemeData(
            colorScheme: scheme,
            fontFamily: 'Roboto',
          ).withAppShapes.withAppTypography;
          await tester.pumpWidget(
            RepaintBoundary(
              key: const ValueKey('snapshot'),
              child: MaterialApp(
                debugShowCheckedModeBanner: false,
                theme: theme,
                home: LiquidGlassScope(
                  enabled: true,
                  child: LiquidGlassHost(
                    enabled: true,
                    child: Stack(
                      children: [
                        Positioned.fill(
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              for (final color in bands)
                                Expanded(child: ColoredBox(color: color)),
                            ],
                          ),
                        ),
                        Positioned(
                          left: 16,
                          top: 16,
                          width: 44,
                          height: 44,
                          child: LiquidGlass(
                            style: LiquidGlassStyle.regular(scheme),
                            borderRadius: BorderRadius.circular(999),
                            child: const Center(
                              child: GlyphIcon(AppGlyphs.refresh, size: 21),
                            ),
                          ),
                        ),
                        Align(
                          alignment: Alignment.bottomCenter,
                          child: NavigationDock(
                            destinations: const [
                              NavigationDockDestination(
                                glyph: AppGlyphs.dashboard,
                                label: 'Dashboard',
                              ),
                              NavigationDockDestination(
                                glyph: AppGlyphs.proxies,
                                label: 'Proxies',
                              ),
                              NavigationDockDestination(
                                glyph: AppGlyphs.profiles,
                                label: 'Profiles',
                              ),
                              NavigationDockDestination(
                                glyph: AppGlyphs.tools,
                                label: 'Tools',
                              ),
                            ],
                            selectedIndex: 0,
                            onSelected: (_) {},
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          );
          await tester.pump();
          await tester.pump(const Duration(seconds: 1));
          TestGesture? gesture;
          if (held) {
            gesture = await tester.startGesture(const Offset(66, 348));
            await tester.pump();
            await tester.pump(const Duration(milliseconds: 600));
          }
          await tester.runAsync(() async {
            final boundary = tester.renderObject<RenderRepaintBoundary>(
              find.byKey(const ValueKey('snapshot')),
            );
            final snapshot = await boundary.toImage(pixelRatio: 1);
            final bytes = await snapshot.toByteData(
              format: ui.ImageByteFormat.png,
            );
            await File(
              '$directory/glass-dock-${dark ? 'dark' : 'light'}-${held ? 'held' : 'rest'}.png',
            ).writeAsBytes(bytes!.buffer.asUint8List());
            snapshot.dispose();
          });
          await gesture?.up();
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pump(const Duration(seconds: 1));
        },
      );
    }
  }
}
