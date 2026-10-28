// Synthetic compatibility test for unchanged FlClash
// 7fd417fb9a9a277c6a9339a8ac87f907df7c25f7 (GPL-3.0).
//
// Generate input with TestFlutterBackupReferenceFixture (internal/store), then
// copy this file to test/ in a THROWAWAY pinned source checkout. Disable BOTH
// pubspec hooks (setup and rust_api), run flutter pub get --enforce-lockfile,
// then restore pubspec/lock/test files byte-for-byte after the test:
//
// FLCLASH_FLUTTER_BACKUP_FIXTURE=/absolute/path/to/build/backup-reference \
// flutter test --no-pub --dart-define=SAFE_MODE=true test/port_backup_test.dart
//
// This never imports main.dart, mounts the app, initializes setup/core managers,
// or runs scripts/compiles rule sets. All app paths and the database are isolated;
// preferences are in memory and HTTP is forbidden. It tests actual original
// BackupAction.restore -> restoreTask/readBackupArchive -> generated models ->
// Database.restore, NOT the native Go reimporter. Retained metrics are synthetic.
import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:drift/native.dart';
import 'package:fl_clash/common/common.dart';
import 'package:fl_clash/common/launch.dart';
import 'package:fl_clash/database/database.dart';
import 'package:fl_clash/enum/enum.dart';
import 'package:fl_clash/l10n/l10n.dart';
import 'package:fl_clash/models/models.dart';
import 'package:fl_clash/providers/action.dart';
import 'package:fl_clash/providers/config.dart';
import 'package:flutter/widgets.dart' show Locale;
import 'package:flutter_test/flutter_test.dart';
import 'package:path/path.dart' as p;
import 'package:riverpod/riverpod.dart';

import 'helpers/app_paths.dart';
import 'helpers/preferences.dart';

class _NoNetwork extends HttpOverrides {
  int attempts = 0;

  @override
  HttpClient createHttpClient(SecurityContext? context) {
    attempts++;
    throw StateError('HTTP is forbidden in the backup reference harness');
  }
}

void main() {
  if (!safeModeBuild) {
    throw StateError('This harness requires --dart-define=SAFE_MODE=true');
  }
  TestWidgetsFlutterBinding.ensureInitialized();
  final fixturePath = Platform.environment['FLCLASH_FLUTTER_BACKUP_FIXTURE'];
  if (fixturePath == null || !p.isAbsolute(fixturePath)) {
    throw StateError('Set FLCLASH_FLUTTER_BACKUP_FIXTURE to an absolute path');
  }
  final fixture = jsonDecode(
    File(p.join(fixturePath, 'expected.json')).readAsStringSync(),
  ) as Map<String, dynamic>;
  if (fixture['synthetic'] != true ||
      fixture['archive'] != 'native-export.zip') {
    throw StateError('Only the synthetic native-export fixture is accepted');
  }
  final archivePath = p.join(fixturePath, fixture['archive'] as String);
  final expectedFiles = (fixture['files'] as Map<String, dynamic>).map(
    (name, value) => MapEntry(name, base64Decode(value as String)),
  );
  final network = _NoNetwork();
  final previousNetwork = HttpOverrides.current;
  final results = <String, Object?>{};
  late Directory sandbox;
  late Directory home;
  late Database db;
  var hasDatabase = false;

  Future<MigrationData> parse() => readBackupArchive(
    archivePath: archivePath,
    stagingDirPath: p.join(sandbox.path, 'parser-staging'),
  );

  void expectModels(MigrationData data) {
    expect(data.profiles.map((v) => v.label), [
      'Source',
      'Scripted',
      'Inline',
      'Custom',
    ]);
    final source = data.profiles[0];
    final scripted = data.profiles[1];
    final inlined = data.profiles[2];
    final custom = data.profiles[3];
    expect(source.type, ProfileType.url);
    expect(source.url, 'https://example.invalid/sub');
    expect(source.autoUpdate, false);
    expect(source.autoUpdateDuration.inMilliseconds, 1234567);
    expect(source.lastUpdateDate!.millisecondsSinceEpoch, 1700000000000);
    expect(
      source.subscriptionInfo,
      const SubscriptionInfo(
        upload: 1234567890123,
        download: 2345,
        total: 9000000000000,
        expire: 1900000000,
      ),
    );
    expect(source.selectedMap, {'Select': 'DIRECT'});
    expect(source.unfoldSet, {'Select'});
    expect(scripted.selectedMap, {'Select': 'REJECT'});
    expect(inlined.selectedMap, {'Select': 'DIRECT'});
    expect(custom.selectedMap, {'Select': 'Node'});
    expect(custom.currentGroupName, 'Select');
    expect(custom.unfoldSet, {'Select'});
    expect(custom.type, ProfileType.custom);
    expect(custom.overrides.dns.ipv6, true);
    expect(custom.overrides.dnsOverrideKeys, contains(DnsOverrideKey.ipv6));
    expect(custom.overrides.proxyProviders.keys, ['Source']);
    expect(custom.overrides.proxyProviders['Source']!.filter, 'synthetic');
    expect(data.scripts.map((v) => v.label), [
      'Shared',
      'Scripted script',
      'Inline script',
    ]);
    expect(scripted.extendType, ExtendType.script);
    expect(scripted.scriptId, data.scripts[0].id);
    expect(inlined.scriptId, data.scripts[2].id);
    expect(data.scripts[1].id, isNot(scripted.scriptId));
    expect(data.proxyGroups.single.profileId, custom.id);
    expect(data.proxyGroups.single.name, 'Select');
    expect(data.proxyGroups.single.proxies, ['Node', 'DIRECT']);
    expect(data.proxyGroups.single.use, ['Source']);
    expect(data.proxyGroups.single.defaultSelected, 'Node');
    expect(data.customProxies.single.definition, {
      'name': 'Node',
      'type': 'socks5',
      'server': '192.0.2.1',
      'port': 1080,
    });
    expect(data.proxyDialers, [
      ProxyDialer(
        profileId: custom.id,
        proxyId: data.customProxies.single.id,
        target: 'DIRECT',
      ),
    ]);
    final expectedRules = {
      source.id: [
        'DOMAIN,added.invalid,DIRECT',
        'IP-CIDR,192.0.2.0/24,DIRECT,src,no-resolve',
      ],
      custom.id: [
        'RULE-SET,Local,Select',
        'RULE-SET,Binary,Select',
        'DOMAIN-REGEX,^a,b\$,DIRECT',
        'MATCH,Select',
      ],
    };
    for (final entry in expectedRules.entries) {
      final rules = data.rules.where((v) => v.profileId == entry.key).toList()
        ..sort((a, b) => a.order!.compareTo(b.order!));
      expect(rules.map((v) => v.rawValue), entry.value);
      // Exercise the original Rule.parse as well as Drift/generated decoding.
      for (final rule in rules) {
        final reparsed = Rule.parse(
          rule.rawValue,
          id: rule.id,
        ).copyWith(profileId: rule.profileId, order: rule.order);
        expect(reparsed, rule);
      }
    }
    expect(data.disabledRules, isEmpty);
    expect(data.clashProviders.map((v) => v.label).toSet(), {
      'Local',
      'Binary',
    });
    final local = data.clashProviders.singleWhere((v) => v.label == 'Local');
    final binary = data.clashProviders.singleWhere((v) => v.label == 'Binary');
    expect(local.behavior, RuleProviderBehavior.classical);
    expect(local.format, RuleProviderFormat.yaml);
    expect(binary.behavior, RuleProviderBehavior.domain);
    expect(binary.format, RuleProviderFormat.mrs);
    expect(data.clashProviders.every((v) => !v.isRemote), true);
    expect(data.configMap!['version'], 1);
    final config = Config.strictFromJson(data.configMap!);
    expect(
      Config.fromJson(data.configMap!),
      config,
      reason: 'a tolerant parse must not silently salvage/drop exported data',
    );
    expect(config.currentProfileId, custom.id);
    expect(config.themeProps.themeMode.name, 'dark');
    expect(config.appSettingProps.floatingNavigationBar, false);
    expect(config.excludeSSIDs, ['synthetic-reference-wifi']);
    expect(config.davProps!.password, 'synthetic-not-a-secret');
  }

  Future<Map<String, Object?>> expectFiles(
    MigrationData data,
    String root,
  ) async {
    final entries = BackupEntries.ofData(data);
    expect(entries.toSet(), expectedFiles.keys.toSet());
    final hashes = <String, String>{};
    for (final entry in entries) {
      final bytes = await File(BackupEntries.resolve(root, entry))
          .readAsBytes();
      expect(bytes, expectedFiles[entry], reason: entry);
      hashes[entry] = sha256.convert(bytes).toString();
    }
    for (final profile in data.profiles.take(3)) {
      expect(
        await File(
          BackupEntries.resolve(root, BackupEntries.profile(profile.id)),
        ).readAsString(),
        fixture['source'],
      );
    }
    for (final script in data.scripts) {
      final key = switch (script.label) {
        'Shared' => 'sharedScript',
        'Inline script' => 'inlineScript',
        _ => 'dormantScript',
      };
      expect(
        await File(BackupEntries.resolve(root, BackupEntries.script(script.id)))
            .readAsString(),
        fixture[key],
      );
    }
    for (final provider in data.clashProviders) {
      final body = await File(
        BackupEntries.resolve(root, BackupEntries.provider(provider)),
      ).readAsBytes();
      expect(
        body,
        provider.label == 'Local'
            ? utf8.encode(fixture['localResource'] as String)
            : base64Decode(fixture['binaryResource'] as String),
      );
    }
    return {'count': entries.length, 'sha256': hashes};
  }

  Future<Map<String, int>> expectDatabase(MigrationData data) async {
    final counts = <String, int>{};
    for (final entry
        in (fixture['rowCounts'] as Map<String, dynamic>).entries) {
      counts[entry.key] =
          (await db
                  .customSelect('SELECT count(*) AS n FROM ${entry.key}')
                  .getSingle())
              .read<int>('n');
      expect(counts[entry.key], entry.value, reason: entry.key);
    }
    expect(await db.profilesDao.query().get(), data.profiles);
    expect(await db.scriptsDao.query().get(), data.scripts);
    expect(await db.customProxiesDao.query().get(), data.customProxies);
    expect(await db.clashProvidersDao.query().get(), data.clashProviders);
    expect(
      await db.proxyDialersDao.query(data.profiles.last.id).get(),
      data.proxyDialers,
    );
    expect(
      await db.proxyGroupsDao.query(data.profiles.last.id).get(),
      data.proxyGroups,
    );
    for (final profile in data.profiles) {
      final expected =
          data.rules.where((v) => v.profileId == profile.id).toList()
            ..sort((a, b) => a.order!.compareTo(b.order!));
      final actual = await db.rulesDao.queryProfileRules(profile.id).get();
      // Original restore reindexes rule order keys; relative order must survive.
      expect(
        actual.map((v) => v.copyWith(order: null)),
        expected.map((v) => v.copyWith(order: null)),
      );
    }
    expect(await db.customSelect('PRAGMA foreign_key_check').get(), isEmpty);
    expect(
      (await db.customSelect('PRAGMA integrity_check').getSingle()).data.values,
      ['ok'],
    );
    return counts;
  }

  setUpAll(() async {
    HttpOverrides.global = network;
    await AppLocalizations.load(const Locale('en'));
    expect(proxy, isNull, reason: 'SAFE_MODE must suppress the proxy manager');
    expect(autoLaunch, isNull);
    expect(
      sha256.convert(await File(archivePath).readAsBytes()).toString(),
      fixture['sha256'],
    );
  });
  setUp(() async {
    sandbox = await Directory.systemTemp.createTemp(
      'flclash-backup-reference-',
    );
    home = await Directory(p.join(sandbox.path, 'home')).create();
    await useAppPaths(home);
    await mockPreferences({'version': 1});
    hasDatabase = false;
  });
  tearDown(() async {
    if (hasDatabase) await db.close();
    await sandbox.delete(recursive: true);
  });
  tearDownAll(() async {
    HttpOverrides.global = previousNetwork;
    expect(network.attempts, 0);
    final sdkRoot = Platform.environment['FLUTTER_ROOT'];
    final sdk = sdkRoot == null
        ? <String, dynamic>{}
        : jsonDecode(
            await File(p.join(sdkRoot, 'bin', 'cache', 'flutter.version.json'))
                .readAsString(),
          ) as Map<String, dynamic>;
    final metrics = {
      'sourceRevision': '7fd417fb9a9a277c6a9339a8ac87f907df7c25f7',
      'workflowFlutterVersion': '3.47.6',
      'actualFlutterVersion': sdk['flutterVersion'],
      'frameworkRevision': sdk['frameworkRevision'],
      'dartVersion': Platform.version,
      'sdkMismatch': sdk['flutterVersion'] != '3.47.6',
      'fixtureSHA256': fixture['sha256'],
      'safeMode': safeModeBuild,
      'httpAttempts': network.attempts,
      'results': results,
      'limits':
          'Synthetic current-schema restore only; no app startup, core, '
          'script execution, MRS compilation, OS proxy/TUN or older-schema evidence.',
    };
    await File(
      p.join(fixturePath, 'flutter-restore-metrics.json'),
    ).writeAsString('${const JsonEncoder.withIndent('  ').convert(metrics)}\n');
  });

  test(
    'original archive parser and generated models accept native export',
    () async {
      final clock = Stopwatch()..start();
      final data = await parse();
      expectModels(data);
      results['parser'] = {
        'passed': true,
        'elapsedMs': clock.elapsedMilliseconds,
        'files': await expectFiles(
          data,
          p.join(sandbox.path, 'parser-staging'),
        ),
      };
    },
  );

  for (final mode in ['compatible-all', 'override-all', 'profiles-only']) {
    test('original BackupAction.restore: $mode', () async {
      final clock = Stopwatch()..start();
      final parsed = await parse();
      expectModels(parsed);
      db = Database(NativeDatabase.memory());
      hasDatabase = true;
      database = db;
      final container = ProviderContainer();
      addTearDown(container.dispose);
      container.listen(configProvider, (_, _) {});
      final before = container.read(configProvider);
      if (mode == 'override-all') {
        await db.profilesDao.putAll([
          const Profile(
            id: 999,
            label: 'Discard this synthetic row',
            autoUpdateDuration: Duration.zero,
          ).toCompanion(),
        ]);
        container
            .read(appSettingProvider.notifier)
            .update(
              (value) =>
                  value.copyWith(restoreStrategy: RestoreStrategy.override),
            );
      }
      final onlyProfiles = mode == 'profiles-only';
      expect(
        await container
            .read(backupActionProvider.notifier)
            .restore(
              onlyProfiles ? RestoreOption.onlyProfiles : RestoreOption.all,
              (_) async => archivePath,
            ),
        true,
      );
      final counts = await expectDatabase(parsed);
      final files = await expectFiles(parsed, home.path);
      final restored = container.read(configProvider);
      if (onlyProfiles) {
        expect(restored, before);
      } else {
        final expected = Config.strictFromJson(parsed.configMap!);
        // writeConfig owns the controller secret; never retain it in metrics.
        expect(
          restored.copyWith(
            patchClashConfig: restored.patchClashConfig.copyWith(
              secret: expected.patchClashConfig.secret,
            ),
          ),
          expected,
        );
      }
      expect(
        home.listSync().where((v) => p.basename(v.path).startsWith('backup')),
        isEmpty,
        reason: 'the action must clean up its work/staging directory',
      );
      results[mode] = {
        'passed': true,
        'elapsedMs': clock.elapsedMilliseconds,
        'rowCounts': counts,
        'files': files,
      };
    });
  }
}
