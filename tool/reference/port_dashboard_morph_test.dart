// Appended (not imported) to an unmodified copy of current_outbound.dart by
// prepare_dashboard_morph.py, so the tests exercise its real private state.
import 'dart:convert';
import 'dart:io';
import 'dart:ui' show PointerDeviceKind;
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:fl_clash/l10n/l10n.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  final result = <String, Object?>{
    'upstream': '7fd417fb9a9a277c6a9339a8ac87f907df7c25f7',
    'flutter': Platform.environment['FLCLASH_FLUTTER_VERSION'],
    'workflowFlutter': '3.47.6',
    'morphMS': _morphDuration.inMilliseconds,
    'stateMS': _stateDuration.inMilliseconds,
  };
  test('source intermediate playPause paths and curves', () {
    result['glyphs'] = [
      for (final progress in [-0.08, 0.0, 0.125, 0.25, 0.5, 0.75, 1.0, 1.08])
        {
          'progress': progress,
          'vertices': [
            for (final shape in AppGlyphs.playPause(progress).shapes.cast<GlyphPolyline>())
              [for (final v in shape.vertices) [v.x, v.y]],
          ],
          'bounds': [
            for (final shape in AppGlyphs.playPause(progress).shapes)
              [shape.path.getBounds().left, shape.path.getBounds().top,
               shape.path.getBounds().width, shape.path.getBounds().height],
          ],
        },
    ];
    result['curves'] = [
      for (final t in [0.0, 0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0])
        {'t': t, 'forward': Curves.easeOutBack.transform(t),
         'reverse': Curves.easeInCubic.transform(t)},
    ];
    result['shapeBounds'] = [
      for (final t in [0.0, 0.25, 0.5, 0.75, 1.0, 1.08])
        {'progress': t, 'bounds': [
          _morphPath(t).getBounds().left, _morphPath(t).getBounds().top,
          _morphPath(t).getBounds().width, _morphPath(t).getBounds().height,
        ]},
    ];
  });
  testWidgets('original control forward reverse interruption press and accessibility', (tester) async {
    bool running = false, suspended = false, reduce = false, enabled = true;
    late StateSetter rebuild;
    final toggle = find.byType(_StartToggle);
    await tester.pumpWidget(ProviderScope(child: MaterialApp(
      locale: const Locale('en'),
      localizationsDelegates: const [AppLocalizations.delegate,
        AppUiLocalizations.delegate, ...GlobalMaterialLocalizations.delegates],
      supportedLocales: AppLocalizations.delegate.supportedLocales,
      home: StatefulBuilder(builder: (context, setState) {
        rebuild = setState;
        return MediaQuery(data: MediaQuery.of(context).copyWith(disableAnimations: reduce),
          child: Scaffold(body: Center(child: _StartToggle(
            size: 88, enabled: enabled, running: running, suspended: suspended))));
      }),
    )));
    Map<String, Object?> sample(int ms) {
      final state = tester.state<_StartToggleState>(toggle);
      final glyph = tester.widget<GlyphIcon>(find.descendant(
        of: toggle, matching: find.byType(GlyphIcon))).glyph;
      final box = tester.renderObject<RenderBox>(find.descendant(
        of: toggle, matching: find.byType(GlyphIcon)));
      final painted = ElasticPress.paintBoundsOf(box);
      final painter = tester.widgetList<CustomPaint>(find.descendant(
        of: toggle, matching: find.byType(CustomPaint))).map((w) => w.painter)
          .whereType<_MorphPainter>().single;
      return {'ms': ms, 'controller': state._controller.value,
        'shape': state._progress.value,
        'vertices': [for (final shape in glyph.shapes)
          if (shape is GlyphPolyline) [for (final v in shape.vertices) [v.x, v.y]]],
        'lift': ElasticPress.liftOf(box),
        'paintBounds': [painted.left, painted.top, painted.width, painted.height],
        'overlay': state._overlay,
        'color': painter.color?.toARGB32(),
        'ring': painter.ring == null ? null : {
          'width': painter.ring!.width, 'align': painter.ring!.strokeAlign,
          'color': painter.ring!.color.toARGB32()},
      };
    }
    Future<List<Object?>> trace() async {
      final rows = <Object?>[sample(0)];
      for (var ms = 45; ms <= 450; ms += 45) {
        await tester.pump(const Duration(milliseconds: 45));
        rows.add(sample(ms));
      }
      return rows;
    }
    rebuild(() => running = true);
    await tester.pump();
    result['forward'] = await trace();
    rebuild(() => running = false);
    await tester.pump();
    result['reverse'] = await trace();
    await tester.pump(const Duration(milliseconds: 1));
    rebuild(() => running = true);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 451));
    rebuild(() => running = false);
    await tester.pump();
    result['settledReverse'] = await trace();
    await tester.pump(const Duration(milliseconds: 1));
    rebuild(() => running = true);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 180));
    rebuild(() => running = false);
    await tester.pump();
    result['interrupted'] = await trace();
    final press = await tester.startGesture(tester.getCenter(toggle));
    await tester.pump();
    final held = <Object?>[sample(0)];
    for (var ms = 30; ms <= 300; ms += 30) {
      await tester.pump(const Duration(milliseconds: 30));
      held.add(sample(ms));
    }
    result['held'] = held;
    await press.cancel(); // Never dispatch the source core action.
    await tester.pump();
    result['release'] = await trace();
    rebuild(() { reduce = true; running = true; });
    await tester.pump();
    result['mediaQueryReduced'] = await trace();
    await tester.pump(const Duration(milliseconds: 1));
    tester.binding.platformDispatcher.accessibilityFeaturesTestValue =
      FakeAccessibilityFeatures(disableAnimations: true);
    rebuild(() => running = false);
    await tester.pump();
    final reducedRows = <Object?>[sample(0)];
    for (var ms = 5; ms <= 30; ms += 5) {
      await tester.pump(const Duration(milliseconds: 5));
      reducedRows.add(sample(ms));
    }
    result['platformReduced'] = reducedRows;
    tester.binding.platformDispatcher.clearAccessibilityFeaturesTestValue();
    rebuild(() => reduce = false);
    await tester.pumpAndSettle();
    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await mouse.addPointer(location: Offset.zero);
    await mouse.moveTo(tester.getCenter(toggle));
    await tester.pump();
    final hover = <Object?>[sample(0)];
    await tester.pump(const Duration(milliseconds: 75));
    hover.add(sample(75));
    await tester.pump(const Duration(milliseconds: 75));
    hover.add(sample(150));
    result['hover'] = hover;
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pumpAndSettle();
    result['focused'] = sample(0);
    await mouse.removePointer();
    rebuild(() { enabled = false; suspended = true; });
    await tester.pumpAndSettle();
    result['disabledSuspended'] = sample(0);
    final output = File(Platform.environment['FLCLASH_DASHBOARD_MORPH_PATH']!);
    output.parent.createSync(recursive: true);
    output.writeAsStringSync(const JsonEncoder.withIndent('  ').convert(result));
    await tester.pumpWidget(const SizedBox.shrink());
  });
}
