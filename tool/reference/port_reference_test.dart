// Copy into the pinned FlClash checkout's test/ directory; see README.md.
// Synthetic fixtures only. FlClash widgets remain unchanged.
import 'dart:io';
import 'dart:async';
import 'dart:convert';
import 'dart:math' as math;
import 'dart:ui' as ui;
import 'dart:typed_data';

import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';
import 'package:fl_clash/enum/enum.dart';
import 'package:fl_clash/models/models.dart';
import 'package:fl_clash/l10n/l10n.dart';
import 'package:fl_clash/providers/providers.dart';
import 'package:fl_clash/providers/ip_quality.dart';
import 'package:fl_clash/providers/outbound_ip.dart';
import 'package:fl_clash/providers/routed_probe.dart';
import 'package:fl_clash/providers/service_status.dart';
import 'package:fl_clash/common/service_probe.dart';
import 'package:fl_clash/common/common.dart';
import 'package:fl_clash/manager/theme_manager.dart';
import 'package:fl_clash/manager/window_manager.dart';
import 'package:fl_clash/state.dart';
import 'package:fl_clash/ui/ui.dart';
import 'package:fl_clash/ui/icons/icons.dart';
import 'package:fl_clash/views/proxies/proxies.dart';
import 'package:fl_clash/views/proxies/common.dart';
import 'package:fl_clash/views/dashboard/dashboard.dart';
import 'package:fl_clash/views/dashboard/widgets/memory_info.dart';
import 'package:fl_clash/views/dashboard/widgets/card.dart';
import 'package:fl_clash/views/dashboard/widgets/profile_detail.dart';
import 'package:fl_clash/views/dashboard/widgets/proxy_groups.dart';
import 'package:fl_clash/views/profiles/profiles.dart';
import 'package:fl_clash/views/dashboard/widgets/core_status_button.dart';
import 'package:fl_clash/views/theme.dart';
import 'package:fl_clash/views/hotkey.dart';

import 'helpers/test_profiles.dart';

const names = [
  'United States 01',
  'Hong Kong 01',
  'Singapore 01',
  'Japan 01',
  'United Kingdom 01',
  'Taiwan 01',
];

class ReferenceBinding extends AutomatedTestWidgetsFlutterBinding {
  @override
  bool get disableShadows => false;
}

class FixtureIPs extends OutboundIpProbe {
  @override
  RoutedProbeState<String, IpInfo> build() => const RoutedProbeState({
    routedOutbound: ProbeEntry(
      phase: ProbePhase.fresh,
      value: IpInfo(ip: '203.0.113.42', countryCode: 'US'),
    ),
  });
  @override
  void watch(String target) {}
  @override
  void unwatch(String target) {}
}

class FixtureServices extends ServiceStatus {
  @override
  RoutedProbeState<ServiceTarget, ServiceCheck> build() =>
      const RoutedProbeState({
        ServiceTarget.google: ProbeEntry(
          phase: ProbePhase.fresh,
          value: ServiceCheck(ServiceProbeStatus.available, delay: 38),
        ),
        ServiceTarget.github: ProbeEntry(
          phase: ProbePhase.fresh,
          value: ServiceCheck(ServiceProbeStatus.available, delay: 64),
        ),
      });
  @override
  void watch(ServiceTarget target) {}
  @override
  void unwatch(ServiceTarget target) {}
}

class ReferenceApp extends ConsumerWidget {
  const ReferenceApp({
    required this.child,
    required this.dark,
    required this.selected,
    this.scale = 1,
    this.expanded = true,
    this.bare = false,
    super.key,
  });
  final Widget child;
  final bool dark;
  final int selected;
  final double scale;
  final bool expanded;
  final bool bare;
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = watchAppTheme(ref, dark ? Brightness.dark : Brightness.light);
    return RepaintBoundary(
      key: const ValueKey('snapshot'),
      child: MaterialApp(
        debugShowCheckedModeBanner: false,
        navigatorKey: globalState.navigatorKey,
        locale: const Locale('en'),
        theme: theme,
        darkTheme: theme,
        themeMode: dark ? ThemeMode.dark : ThemeMode.light,
        localizationsDelegates: const [
          AppLocalizations.delegate,
          AppUiLocalizations.delegate,
          ...GlobalMaterialLocalizations.delegates,
        ],
        supportedLocales: AppLocalizations.delegate.supportedLocales,
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(context).copyWith(textScaler: TextScaler.linear(scale)),
          child: Builder(builder: (context) {
            final measure = Measure.of(context, scale);
            globalState.measure = measure;
            return MeasureScope(measure: measure, child: AppViewScope(child: child!));
          }),
        ),
        home: Scaffold(
          body: bare ? child : Row(
            children: [
              ColoredBox(
                color: theme.colorScheme.chromeSurface,
                child: NavigationSidebar(
                  destinations: const [
                    SidebarDestination(
                      glyph: AppGlyphs.dashboard,
                      label: 'Dashboard',
                    ),
                    SidebarDestination(
                      glyph: AppGlyphs.proxies,
                      label: 'Proxies',
                    ),
                    SidebarDestination(
                      glyph: AppGlyphs.profiles,
                      label: 'Profiles',
                    ),
                    SidebarDestination(
                      glyph: AppGlyphs.connections,
                      label: 'Live connections',
                    ),
                    SidebarDestination(glyph: AppGlyphs.tools, label: 'Tools'),
                  ],
                  selectedIndex: selected,
                  expanded: expanded,
                  onSelected: (_) {},
                  onToggle: () {},
                  windowControls: Size.zero,
                ),
              ),
              Expanded(child: child),
            ],
          ),
        ),
      ),
    );
  }
}

void main() {
  ReferenceBinding();
  setUpAll(() async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          const MethodChannel('plugins.flutter.io/path_provider'),
          (call) async => Directory.systemTemp.path,
        );

    final sdk = Platform.environment['FLUTTER_ROOT']!;
    for (final entry in const [
      ('Roboto-Regular.ttf', FontWeight.w400),
      ('Roboto-Medium.ttf', FontWeight.w500),
      ('Roboto-Bold.ttf', FontWeight.w700),
    ]) {
      final data = await File(
        '$sdk/bin/cache/artifacts/material_fonts/${entry.$1}',
      ).readAsBytes();
      await (FontLoader(
        'Roboto',
      )..addFont(Future.value(ByteData.sublistView(data)))).load();
    }
  });
  test('dashboard settings migration', () async {
    final cases = <List<String>?>[
      null, [], ['runTime', 'overrideDnsButton'], ['unknown'],
      ['networkSpeed','systemProxyButton','tunButton','outboundModeV2','networkDetection','trafficUsage','intranetIp'],
      ['requests','connections','trafficUsage','systemProxyButton'],
      ['memoryInfo','currentOutbound','memoryInfo','unknown','vpnButton'],
      ['profiles','networkDetection','outboundModeV2'],
    ];
    await File('${Platform.environment['FLCLASH_REFERENCE_DIR']!}/dashboard-widgets.json').writeAsString(jsonEncode([
      for (final value in cases) {'input':value,'output':dashboardWidgetsFromJson(value).map((e)=>e.name).toList()},
    ]));
  });
  test('compact provider counts', () async {
    final values = [0, 1, 9, 999, 1000, 1001, 1234, 9994, 9999, 12345, 123456, 999499, 999500, 999999, 1000000, 1234567, -1234];
    await File('${Platform.environment['FLCLASH_REFERENCE_DIR']!}/compact-counts.json').writeAsString(jsonEncode([
      for (final value in values) {'value': value, 'label': value.compact},
    ]));
  });
  for (final page in [
    'proxies-list',
    'proxies-tab',
    'proxies-icons',
    'proxies-pending',
    'core-connected',
    'core-connecting',
    'core-disconnected',
    'sidebar-collapsed',
    'dashboard',
    'dashboard-short',
    'dashboard-two',
    'dashboard-standard',
    'dashboard-medium',
    'dashboard-narrow',
    'dashboard-narrow-edit',
    'dashboard-narrow-custom',
    'dashboard-narrow-scaled-up',
    'dashboard-narrow-scaled-down',
    'dashboard-scaled-up',
    'dashboard-scaled-down',
    'proxies-scaled-up',
    'proxies-scaled-down',
    'theme',
    'hotkeys',
    'quality-sheet',
    'memory-sheet',
    'memory-stopped',
    'profile-picker-small',
    'profile-picker-small-many',
    'proxy-picker-small',
    'profile-details-small',
    'profile-picker',
    'profile-picker-many',
    'profile-grid',
    'proxy-picker',
    'proxy-picker-tab',
    'profile-details',
    'profile-details-subscription',
    'profile-details-providers',
    'caption-normal',
    'caption-maximized',
    'caption-pinned-fullscreen',
    'caption-close-hover',
    'services-sheet',
    'services-manage',
  ]) {
    for (final dark in [false, true]) {
      testWidgets('$page ${dark ? 'dark' : 'light'}', (tester) async {
        final scale = page.endsWith('scaled-up') ? 1.4 : page.endsWith('scaled-down') ? 0.8 : 1.0;
        final size = switch (page) {
          'profile-picker-small' || 'profile-picker-small-many' || 'proxy-picker-small' || 'profile-details-small' => const Size(560,800),
          'caption-normal' || 'caption-maximized' || 'caption-pinned-fullscreen' || 'caption-close-hover' => const Size(1200,32),
          'dashboard-short' => const Size(1200, 640),
          'dashboard-two' => const Size(1000, 800),
          'dashboard-standard' => const Size(1440, 1000),
          'dashboard-medium' => const Size(1440, 860),
          'dashboard-narrow' || 'dashboard-narrow-edit' || 'dashboard-narrow-custom' || 'dashboard-narrow-scaled-up' || 'dashboard-narrow-scaled-down' => const Size(820, 800),
          _ => const Size(1200, 800),
        };
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final profile = Profile(
          id: 1,
          label: 'Example',
          subscriptionInfo: page == 'profile-details-subscription' ? const SubscriptionInfo(upload: 1073741824, download: 44023414784, total: 107374182400, expire: 1893456000) : null,
          type: ProfileType.url,
          url: 'https://example.com/subscription',
          autoUpdateDuration: const Duration(days: 1),
          currentGroupName: 'Proxy',
          selectedMap: {
            'Proxy': names[0],
            'Auto': names[1],
            'Streaming': names[2],
          },
          unfoldSet: const {'Proxy', 'Auto', 'Streaming'},
        );
        final iconFixture = page == 'proxies-icons'
          ? (jsonDecode(File(Platform.environment['FLCLASH_ICON_FIXTURE']!).readAsStringSync()) as List).cast<String>()
          : <String>['','',''];
        final groups = [
          for (var i = 0; i < 3; i++)
            Group(
              name: ['Proxy', 'Auto', 'Streaming'][i],
              icon: iconFixture[i],
              type: i == 1 ? GroupType.URLTest : GroupType.Selector,
              hidden: false,
              now: names[i],
              all: [for (final n in names) Proxy(name: n, type: 'Shadowsocks')],
            ),
        ];
        final container = ProviderContainer(
          overrides: [
            providersProvider.overrideWithBuild((_, _) => page == 'profile-details-providers' ? [
              ExternalProvider(name:'Remote nodes', type:'Proxy', vehicleType:'HTTP', count:1234, path:'/tmp/flclash-reference-unused-provider.yaml', updateAt:DateTime.fromMillisecondsSinceEpoch(0)),
              ExternalProvider(name:'Domain rules', type:'Rule', vehicleType:'File', count:123456, path:'/tmp/flclash-reference-unused-rules.yaml', updateAt:DateTime.fromMillisecondsSinceEpoch(0)),
              ExternalProvider(name:'Subscription nodes', type:'Proxy', vehicleType:'HTTP', count:1234, updateAt:DateTime.fromMillisecondsSinceEpoch(0), subscriptionInfo:const SubscriptionInfo(upload:1073741824,download:44023414784,total:107374182400,expire:1893456000)),
            ] : []),
            profilesProvider.overrideWith(() => TestProfiles([profile, if (page == 'profile-picker-many' || page == 'profile-picker-small-many' || page == 'profile-grid') ...[Profile(autoUpdateDuration:const Duration(days:1),id:2,label:'Local configuration',type:ProfileType.file),Profile(autoUpdateDuration:const Duration(days:1),id:3,label:'Second subscription',type:ProfileType.url,url:'https://example.org/subscription',subscriptionInfo:const SubscriptionInfo(upload:1073741824,download:44023414784,total:107374182400,expire:1893456000))]])),
            coreStatusProvider.overrideWithBuild((_, _) => switch (page) {
              'core-connected' => CoreStatus.connected,
              'core-connecting' => CoreStatus.connecting,
              _ => CoreStatus.disconnected,
            }),
            currentProfileIdProvider.overrideWithBuild((_, _) => 1),
            groupsProvider.overrideWithValue(groups),
            currentGroupsStateProvider.overrideWithValue(
              GroupsState(value: groups),
            ),
            appSettingProvider.overrideWithBuild(
              (_, _) => AppSettingProps(locale: 'en', dashboardWidgets: page == 'dashboard-narrow-custom' ? const [DashboardWidget.currentOutbound, DashboardWidget.memoryInfo, DashboardWidget.profiles] : defaultDashboardWidgets),
            ),
            themeSettingProvider.overrideWithBuild(
              (_, _) => ThemeProps(
                primaryColor: 0xff007aff,
                themeMode: dark ? ThemeMode.dark : ThemeMode.light,
              ),
            ),
            proxiesStyleSettingProvider.overrideWithBuild(
              (_, _) => ProxiesStyleProps(
                type: page == 'proxies-tab' || page == 'proxy-picker-tab'
                    ? ProxiesType.tab
                    : ProxiesType.list,
              ),
            ),
            delayDataSourceProvider.overrideWithBuild(
              (_, _) => {
                defaultTestUrl: {
                  for (var i = 0; i < names.length; i++) names[i]: 38 + i * 17,
                },
              },
            ),
            runTimeProvider.overrideWithBuild((_, _) => 8037000),
            outboundIpProbeProvider.overrideWith(FixtureIPs.new),
            serviceStatusProvider.overrideWith(FixtureServices.new),
            ipQualityProvider.overrideWith(
              (_, ip) async => IpQuality(
                ip: ip,
                source: IpQualitySource.ipLocate,
                type: IpType.residential,
                organization: 'Example Broadband',
                asn: 64500,
              ),
            ),
          ],
        );
        if (page == 'proxies-pending') {
          container.read(pendingDelayTestsProvider.notifier).apply(
            acquired: [for (final name in names) delayTestKey(defaultTestUrl, name)],
            started: [for (final name in names.take(2)) delayTestKey(defaultTestUrl, name)],
          );
          container.read(delayTestingGroupsProvider.notifier).start('Proxy');
        }
        globalState.container = container;
        addTearDown(container.dispose);
        container.read(viewSizeProvider.notifier).value = size;
        container.read(sideWidthProvider.notifier).value = page.contains('small') ? 0 : page == 'sidebar-collapsed' ? 48 : 220;
        for (var i = 0; i < 30; i++)
          container
              .read(trafficsProvider.notifier)
              .addTraffic(
                Traffic(
                  up: 35e3 + 15e3 * math.sin(i / 2.1) + 8e3 * math.sin(i / 1.2),
                  down:
                      1.3e6 +
                      0.6e6 * math.sin(i / 2.8) +
                      0.3e6 * math.cos(i / 1.4),
                ),
              );
        container.read(totalTrafficProvider.notifier).value = const Traffic(
          up: 1288490189,
          down: 9234179686,
        );
        final child = page.startsWith('caption-')
            ? WindowHeaderBar(height:32,onDragStart:(){},onDoubleTap:(){},actions:WindowHeaderActions(
                state:ValueNotifier(WindowCaptionState(isMaximized:page=='caption-maximized',isPinned:page=='caption-pinned-fullscreen',isFullScreen:page=='caption-pinned-fullscreen')),
                onPin:(){},onMinimize:(){},onMaximize:(){},onClose:(){},
              ))
            : page.startsWith('core-') || page == 'sidebar-collapsed'
            ? CommonScaffold(title: 'Dashboard', actions: const [CoreStatusButton()], body: const SizedBox.shrink())
            : page.startsWith('proxies')
            ? const ProxiesView()
            : page == 'profile-grid'
            ? const ProfilesView()
            : page == 'theme'
            ? const ThemeView()
            : page == 'hotkeys'
            ? const HotKeyView()
            : const DashboardView(platform: SupportPlatform.Linux);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: ReferenceApp(
              child: child,
              dark: dark,
              selected: page.startsWith('proxies') ? 1 : page == 'profile-grid' ? 2 : (page == 'theme' || page == 'hotkeys') ? 4 : 0,
              scale: scale,
              expanded: page != 'sidebar-collapsed',
              bare: page.startsWith('caption-') || page.contains('small'),
            ),
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(seconds: 2));
        if (page == 'caption-close-hover') {
          final mouse = await tester.createGesture(kind: ui.PointerDeviceKind.mouse);
          await mouse.addPointer(location: const Offset(1000,16));
          await mouse.moveTo(const Offset(1177,16));
          await tester.pump(const Duration(milliseconds:250));
        }
        if (page == 'proxies-icons') {
          final images = tester.widgetList<Image>(find.byType(Image)).toList();
          await tester.runAsync(() async {
            for (final image in images) {
              final stream = image.image.resolve(ImageConfiguration.empty);
              final ready = Completer<void>();
              final listener = ImageStreamListener((_, _) { if (!ready.isCompleted) ready.complete(); }, onError: (Object error, StackTrace? stack) { if (!ready.isCompleted) ready.completeError(error, stack); });
              stream.addListener(listener);
              try { await ready.future; } finally { stream.removeListener(listener); }
            }
          });
          await tester.pump();
        }
        if (page.startsWith('profile-picker')) {
          showProfilesSheet(tester.element(find.byType(DashboardView)));
          await tester.pump();
          await tester.pump(const Duration(milliseconds:500));
        } else if (page.startsWith('proxy-picker')) {
          showProxiesSheet(tester.element(find.byType(DashboardView)));
          await tester.pump();
          await tester.pump(const Duration(milliseconds:500));
        } else if (page.startsWith('profile-details')) {
          showProfileDetailSheet(tester.element(find.byType(DashboardView)));
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 500));
        } else if (page == 'dashboard-narrow-edit') {
          await tester.tap(find.byTooltip('Edit dashboard'));
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 500));
        } else if (page.startsWith('memory-')) {
          final core = page == 'memory-stopped' ? null : const CoreMemoryStats(rss: 48444211, heapInuse: 36700160, heapIdle: 8388608, stackInuse: 2097152, runtimeOther: 1048576);
          final memory = MemorySnapshot(app: 191574835, coreTotal: core?.rss ?? 0, core: core);
          showSheet<void>(context: tester.element(find.byType(DashboardView)), builder: (_) => MemoryDetailSheet(initial: memory, memoryReader: () async => memory));
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 500));
        } else if (page == 'quality-sheet') {
          await tester.tap(find.text('203.0.113.42').first);
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 500));
        } else if (page.startsWith('services-')) {
          await tester.tap(find.byTooltip('Service status').last);
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 500));
          if (page == 'services-manage') {
            await tester.tap(find.byTooltip('Manage services'));
            await tester.pump();
            for (var i = 0; i < 20; i++) {
              await tester.pump(const Duration(milliseconds: 100));
            }
            expect(find.text('Manage services'), findsOneWidget);
          }
        }
        final output = Platform.environment['FLCLASH_REFERENCE_DIR']!;
        final metrics = {
          'scale': scale,
          'profileCards': [for(final element in find.byType(ProfileItem).evaluate()) (() {final r=tester.getRect(find.byElementPredicate((e)=>e==element));return [r.left,r.top,r.width,r.height];})()],
          'captionButtons': [for (final element in find.byType(IconButton).evaluate()) (() { final r = tester.getRect(find.byElementPredicate((e) => e == element)); return [r.left,r.top,r.width,r.height]; })()],
          'size': [size.width, size.height],
          'editButton': [for (final e in find.byTooltip('Edit dashboard').evaluate()) (() {final r=tester.getRect(find.byElementPredicate((v)=>v==e));return [r.left,r.top,r.width,r.height];})()],
          'doneButton': [for (final e in find.byTooltip('Done').evaluate()) (() {final r=tester.getRect(find.byElementPredicate((v)=>v==e));return [r.left,r.top,r.width,r.height];})()],
          'sidebarButtons': [for (final element in find.descendant(of: find.byType(NavigationSidebar), matching: find.byType(InkWell)).evaluate()) (() { final r = tester.getRect(find.byElementPredicate((e) => e == element)); return [r.left, r.top, r.width, r.height]; })()],
          'dashboardCards': [for (final element in find.byType(DashboardCard).evaluate()) (() { final r = tester.getRect(find.byElementPredicate((e) => e == element)); return [r.left, r.top, r.width, r.height]; })()],
          'header': listHeaderHeight,
          'cards': { for (final type in ProxyCardType.values) type.name: getItemHeight(type) },
          'text': { for (final style in AppTextStyle.values) style.name: globalState.measure.textHeight(style) },
        };
        await tester.runAsync(() async {
          final boundary = tester.renderObject<RenderRepaintBoundary>(
            find.byKey(const ValueKey('snapshot')),
          );
          final image = await boundary.toImage(pixelRatio: 1);
          final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
          await Directory(output).create(recursive: true);
          await File('$output/$page-${dark ? 'dark' : 'light'}.png')
              .writeAsBytes(bytes!.buffer.asUint8List());
          await File('$output/$page-${dark ? 'dark' : 'light'}.json').writeAsString(jsonEncode(metrics));
          image.dispose();
        });
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pump(const Duration(seconds: 5));
      });
    }
  }
}
