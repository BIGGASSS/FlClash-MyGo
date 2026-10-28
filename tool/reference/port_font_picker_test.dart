// Safe, synthetic references for the pinned FontFamilyView/SelectionSheet.
// Copy alongside port_reference_test.dart into a prepared source checkout.
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:fl_clash/common/search.dart';
import 'package:fl_clash/common/system_fonts.dart';
import 'package:fl_clash/models/models.dart';
import 'package:fl_clash/providers/providers.dart';
import 'package:fl_clash/state.dart';
import 'package:fl_clash/views/font_family.dart';
import 'package:fl_clash/views/theme.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';

import 'port_reference_test.dart' show ReferenceApp, ReferenceBinding;

void main() {
  ReferenceBinding();
  final directory = Platform.environment['FLCLASH_FONT_REFERENCE_DIR']!;
  setUpAll(() async {
    Directory(directory).createSync(recursive: true);
    // Widget tests default to Ahem, even for a null family. Give that test
    // alias Roboto's real metrics. Synthetic preview names use the same fixture
    // face; style-family assertions below, not these pixels, check assignment.
    final data = await File(
      '${Platform.environment['FLUTTER_ROOT']!}/bin/cache/artifacts/material_fonts/Roboto-Regular.ttf',
    ).readAsBytes();
    for (final alias in [
      'Ahem',
      'Roboto',
      'Alpha Sans',
      'Beta Serif',
      'Gamma Mono',
      'Delta Sans',
      'Epsilon Serif',
      'Eta Sans',
      'Iota Mono',
    ]) {
      await (FontLoader(
        alias,
      )..addFont(Future.value(ByteData.sublistView(data)))).load();
    }
  });
  test('export source sort and AND search', () async {
    final inputs = <List<String>>[
      [],
      [
        ' beta Serif ',
        '.Hidden',
        '',
        'Alpha Sans',
        'alpha sans',
        'BETA SERIF',
        ' 日本語 ',
        'École',
        'école',
      ],
      [
        'Zulu',
        'Roboto',
        'Default',
        'JetBrains Mono',
        '\t',
        '.AppleSystemUIFont',
      ],
      ['𐀀 Family', '\ue000 Family', '😀 Family', '日本語'],
    ];
    final families = sortFontFamilies(inputs[1]);
    final searches = [
      for (final query in [
        '',
        ' SANS \t alPha ',
        'DEFA',
        '  日本 ',
        'missing',
        'Installed fonts',
      ])
        {
          'families': families,
          'query': query,
          'defaultLabel': 'Default',
          'output': [
            if (SearchQuery(query).matches(['Default'])) '',
            ...families.whereMatches(SearchQuery(query), (family) => [family]),
          ],
        },
      {
        'families': ['日本語'],
        'query': '默认',
        'defaultLabel': '默认',
        'output': [''],
      },
    ];
    await File('$directory/font-picker.json').writeAsString(
      const JsonEncoder.withIndent('  ').convert({
        'source': 'FlClash 7fd417fb9a9a277c6a9339a8ac87f907df7c25f7',
        'flutter': '3.47.5',
        'sortCases': [
          for (final input in inputs)
            {'input': input, 'output': sortFontFamilies(input)},
        ],
        'searchCases': searches,
      }),
    );
  });
  for (final dark in [false, true]) {
    testWidgets(
      'font sheet ${dark ? 'dark' : 'light'} stays open on selection',
      (tester) async {
        const size = Size(1200, 800);
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final oldReader = systemFontFamiliesReader;
        addTearDown(() => systemFontFamiliesReader = oldReader);
        // Synthetic names exercise lazy preview styles; absent faces fall back.
        systemFontFamiliesReader = () async => sortFontFamilies([
          'Alpha Sans',
          'Beta Serif',
          'Gamma Mono',
          'Delta Sans',
          'Epsilon Serif',
          'Eta Sans',
          'Iota Mono',
          'JetBrainsMono',
          'Roboto',
          '日本語',
        ]);
        final container = ProviderContainer(
          overrides: [
            appSettingProvider.overrideWithBuild(
              (_, _) => const AppSettingProps(locale: 'en'),
            ),
            themeSettingProvider.overrideWithBuild(
              (_, _) => ThemeProps(
                primaryColor: 0xff007aff,
                themeMode: dark ? ThemeMode.dark : ThemeMode.light,
              ),
            ),
          ],
        );
        globalState.container = container;
        addTearDown(container.dispose);
        container.read(viewSizeProvider.notifier).value = size;
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: ReferenceApp(
              child: const ThemeView(),
              dark: dark,
              selected: 4,
            ),
          ),
        );
        await tester.pumpAndSettle();
        final familyCard = find.byWidgetPredicate(
          (widget) =>
              widget is Text &&
              widget.data == 'Aa' &&
              widget.style?.fontSize == 17,
        );
        await tester.scrollUntilVisible(
          familyCard,
          400,
          scrollable: find
              .descendant(
                of: find.byType(ThemeView),
                matching: find.byType(Scrollable),
              )
              .first,
        );
        await tester.tap(familyCard);
        await tester.pumpAndSettle();
        await tester.pump(const Duration(seconds: 1));
        await tester.pumpAndSettle();
        Finder inPicker(String label) => find.descendant(
          of: find.byType(FontFamilyView),
          matching: find.text(label),
        );
        expect(inPicker('Default'), findsOneWidget);
        expect(inPicker('Installed fonts'), findsOneWidget);
        expect(
          DefaultTextStyle.of(tester.element(inPicker('Beta Serif')))
              .style
              .fontFamily,
          'Beta Serif',
        );
        await tester.tap(inPicker('Beta Serif'));
        await tester.pumpAndSettle();
        expect(container.read(themeSettingProvider).fontFamily, 'Beta Serif');
        expect(find.byType(FontFamilyView), findsOneWidget);
        Future<void> snapshot(String name) async {
          await tester.runAsync(() async {
            final boundary = tester.renderObject<RenderRepaintBoundary>(
              find.byKey(const ValueKey('snapshot')),
            );
            final image = await boundary.toImage(pixelRatio: 1);
            final data = await image.toByteData(format: ui.ImageByteFormat.png);
            await File('$directory/$name-${dark ? 'dark' : 'light'}.png')
                .writeAsBytes(data!.buffer.asUint8List());
            image.dispose();
          });
        }

        await snapshot('font-picker-selected');
        await tester.tap(inPicker('Default'));
        await tester.pumpAndSettle();
        expect(container.read(themeSettingProvider).fontFamily, isNull);
        expect(find.byType(FontFamilyView), findsOneWidget);
        await snapshot('font-picker-default');
        // The source's keyboard search shortcut and section filtering.
        await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
        await tester.sendKeyEvent(LogicalKeyboardKey.keyF);
        await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
        await tester.pumpAndSettle();
        final search = find.descendant(
          of: find.byType(FontFamilyView),
          matching: find.byType(TextField),
        );
        expect(search, findsOneWidget);
        await tester.enterText(search, 'sans alpha');
        await tester.pumpAndSettle();
        expect(inPicker('Alpha Sans'), findsOneWidget);
        expect(inPicker('Beta Serif'), findsNothing);
        expect(inPicker('Default'), findsNothing);
        await snapshot('font-picker-search');
      },
    );
  }
}
